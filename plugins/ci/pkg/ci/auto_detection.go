package ci

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"

	"github.com/ghodss/yaml" // supports both yaml and json
)

var supportedKubeExtensions = []string{"yaml", "yml", "json"}
var supportedExtensions = []string{"yaml", "yml", "json", "tf"}

type KubernetesManifest struct {
	ApiVersion *string `json:"apiVersion"` // Affects YAML field names too.
	Kind       *string `json:"kind"`
}

// ConfigFileAutoDetection reads recursively a path looking for kubernetes manifests and helm charts, returns a fairwinds-insights configuration struct or error
func ConfigFileAutoDetection(basePath string) (*models.Configuration, error) {
	k8sManifests := []string{}
	helmFolders := []string{}

	err := filepath.Walk(basePath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("Could not walk into dir: %v", err)
			}
			if info.IsDir() {
				if strings.HasPrefix(info.Name(), ".") { // hidden
					logrus.Infof("this is a hidden directory: %s, skipping...", info.Name())
					return filepath.SkipDir
				}

				helmFolder, err := isHelmBaseFolder(path)
				if err != nil {
					return err
				}
				if helmFolder {
					relPath, err := filepath.Rel(basePath, path)
					if err != nil {
						return err
					}
					helmFolders = append(helmFolders, relPath)
					return filepath.SkipDir
				}
				logrus.Debugf("this is a directory: %s", info.Name())
				return nil
			}

			fileExtension := filepath.Ext(path)
			if fileExtension == "" {
				return nil
			}

			if !lo.Contains(supportedExtensions, fileExtension[1:]) {
				logrus.Debugf("file extension '%s' not supported for file %v", fileExtension, path)
				return nil
			}

			if isFluxManifest(path) {
				logrus.Debugf("file '%s' is a flux manifest, skipping...", path)
				return nil
			}

			if !isKubernetesManifest(path) {
				logrus.Debugf("file %s is NOT a k8s manifest, skipping...", path)
				return nil
			}

			relPath, err := filepath.Rel(basePath, path)
			if err != nil {
				return err
			}
			logrus.Debugf("it is a k8s manifest: %s", path)
			k8sManifests = append(k8sManifests, relPath)
			return nil
		})

	if err != nil {
		return nil, err
	}

	config := models.Configuration{
		Manifests: models.ManifestConfig{
			YamlPaths: k8sManifests,
			Helm:      toHelmConfigs(basePath, helmFolders),
		},
	}
	return &config, nil
}

func isFluxManifest(path string) bool {
	k8sManifest := getPossibleKubernetesManifest(path)
	if k8sManifest == nil {
		return false
	}
	return strings.Contains(*k8sManifest.ApiVersion, "toolkit.fluxcd.io")
}

func isKubernetesManifest(path string) bool {
	return getPossibleKubernetesManifest(path) != nil
}

// getPossibleKubernetesManifest returns a kubernetesManifest from given path, nil if could not be open or parsed
func getPossibleKubernetesManifest(path string) *KubernetesManifest {
	file, err := os.Open(path)
	if err != nil {
		logrus.Debugf("Could not open file %s", path)
		return nil
	}
	content, err := io.ReadAll(file)
	if err != nil {
		logrus.Debugf("Could not read contents from file %s", file.Name())
		return nil
	}
	var k8sManifest KubernetesManifest
	err = yaml.Unmarshal(content, &k8sManifest)
	if err != nil {
		// not being to unmarshal means it is not a k8s file
		return nil
	}
	if k8sManifest.ApiVersion == nil || k8sManifest.Kind == nil {
		// not having either apiVersion nor kind means it is not a k8s file
		return nil
	}
	return &k8sManifest
}

func isHelmBaseFolder(path string) (bool, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return false, fmt.Errorf("Could not read dir %s: %v", path, err)
	}

	for _, file := range files {
		for _, ext := range supportedKubeExtensions {
			if file.Name() == "Chart."+ext {
				return true, nil
			}
		}
	}
	return false, nil
}

func toHelmConfigs(baseFolder string, helmPaths []string) []models.HelmConfig {
	result := []models.HelmConfig{}
	for _, path := range helmPaths {
		name := tryFetchNameFromChartFile(baseFolder, path)
		hc := models.HelmConfig{
			Name: name,
			Path: path,
		}
		valuesFilePath := tryDiscoverValuesFile(baseFolder, path)
		if valuesFilePath != "" {
			hc.ValuesFile = valuesFilePath
		} else {
			// if default values file does not exists, use a empty map as values
			hc.Values = map[string]interface{}{}
		}
		result = append(result, hc)
	}
	logDuplicatedHelmConfigNames(result)
	return result
}

func logDuplicatedHelmConfigNames(arr []models.HelmConfig) {
	visited := map[string]bool{}
	for _, hc := range arr {
		if _, ok := visited[hc.Name]; ok {
			logrus.Warnf("helm config name '%s' is duplicated", hc.Name)
		} else {
			visited[hc.Name] = true
		}
	}

}

// tries to extract name from Chart.yaml file, return chart (dir name) as fallback
func tryFetchNameFromChartFile(baseFolder, chart string) string {
	for _, ext := range supportedKubeExtensions {
		f := "Chart." + ext
		file, err := os.Open(filepath.Join(baseFolder, chart, f))
		if err != nil {
			logrus.Debugf("Could not open file %s: %v", f, err)
			continue
		}
		content, err := io.ReadAll(file)
		if err != nil {
			logrus.Warnf("Could not read %s: %v", f, err)
			continue
		}

		var s map[string]interface{}
		err = yaml.Unmarshal(content, &s)
		if err != nil {
			logrus.Warnf("Could not unmarshal %s: %v", string(content), err)
			continue
		}

		name, ok := s["name"].(string)
		if !ok {
			logrus.Warnf("Could not get name from file %s: %v", string(content), err)
			continue
		}
		return name
	}
	return chart
}

// tries to discover the default values file, returns empty str if not found
func tryDiscoverValuesFile(baseFolder, path string) string {
	for _, ext := range supportedKubeExtensions {
		f := "values." + ext
		possibleValuesFile := filepath.Join(baseFolder, path, f)
		if _, err := os.Stat(possibleValuesFile); errors.Is(err, os.ErrNotExist) {
			continue
		}
		relPath, _ := filepath.Rel(baseFolder, possibleValuesFile)
		return relPath
	}
	return ""
}

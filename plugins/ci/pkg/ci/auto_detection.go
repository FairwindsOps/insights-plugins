package ci

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/fairwindsops/insights-plugins/ci/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/thoas/go-funk"

	"github.com/ghodss/yaml" // supports both yaml and json
)

var supportedExtentions = []string{"yaml", "yml", "json"}

type KubernetesManifest struct {
	ApiVersion string `json:"apiVersion"` // Affects YAML field names too.
	Kind       int    `json:"kind"`
}

func configFileAutoDetection(baseRepoPath string) (*models.Configuration, error) {
	k8sManifests := []string{}
	helmFolders := []string{}

	err := filepath.Walk(baseRepoPath,
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
					relPath, err := filepath.Rel(baseRepoPath, path)
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

			if !funk.ContainsString(supportedExtentions, fileExtension[1:]) {
				logrus.Debugf("file extention '%s' not supported for file %v", fileExtension, path)
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("Could not open file: %v", err)
			}
			content, err := ioutil.ReadAll(file)
			if err != nil {
				return fmt.Errorf("Could not read fairwinds-insights.yaml: %v", err)
			}

			var kManifest KubernetesManifest
			err = yaml.Unmarshal(content, &kManifest)
			if err != nil {
				relPath, err := filepath.Rel(baseRepoPath, path)
				if err != nil {
					return err
				}
				k8sManifests = append(k8sManifests, relPath)
			}
			return nil
		})

	if err != nil {
		logrus.Error(err)
	}

	config := models.Configuration{
		Manifests: models.ManifestConfig{
			YamlPaths: k8sManifests,
			Helm:      toHelmConfigs(baseRepoPath, helmFolders),
		},
	}

	return &config, nil
}

func isHelmBaseFolder(path string) (bool, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return false, fmt.Errorf("Could not read dir %s: %v", path, err)
	}

	for _, file := range files {
		if file.Name() == "Chart.yaml" || file.Name() == "Chart.yml" || file.Name() == "Chart.json" {
			return true, nil
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
	return result
}

// tries to extract name from Chart.yaml file, return chart (dir name) as fallback
func tryFetchNameFromChartFile(baseFolder, chart string) string {
	chartFiles := []string{"Chart.yaml", "Chart.yml", "Chart.json"}
	for _, f := range chartFiles {
		file, err := os.Open(filepath.Join(baseFolder, chart, f))
		if err != nil {
			logrus.Debugf("Could not open file %s: %v", f, err)
			continue
		}
		content, err := ioutil.ReadAll(file)
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
	possibleValuesFiles := []string{"values.yaml", "values.yml", "values.json"}
	for _, f := range possibleValuesFiles {
		possibleValuesFile := filepath.Join(baseFolder, path, f)
		if _, err := os.Stat(possibleValuesFile); errors.Is(err, os.ErrNotExist) {
			continue
		}
		relPath, _ := filepath.Rel(baseFolder, possibleValuesFile)
		return relPath
	}
	return ""
}

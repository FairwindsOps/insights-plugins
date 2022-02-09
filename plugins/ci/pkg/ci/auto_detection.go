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

				logrus.Infof("this is a directory: %s", info.Name())
				return nil
			}

			fileExtension := filepath.Ext(path)
			if fileExtension == "" {
				return nil
			}

			if !funk.ContainsString(supportedExtentions, fileExtension[1:]) {
				logrus.Infof("file extention '%s' not supported for file %v", fileExtension, path)
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

	logrus.Info("helmFolders: %v", helmFolders)
	logrus.Info("k8sManifests: %v", k8sManifests)

	config := models.Configuration{
		Manifests: models.ManifestConfig{
			YamlPaths: k8sManifests,
			Helm:      toHelmConfigs(baseRepoPath, helmFolders),
		},
	}

	logrus.Info("config: %+v", config)
	return &config, nil
}

func isHelmBaseFolder(path string) (bool, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return false, fmt.Errorf("Could not read dir %s: %v", path, err)
	}

	for _, file := range files {
		if file.Name() == "Chart.yaml" || file.Name() == "Chart.yml" {
			return true, nil
		}
	}
	return false, nil
}

func toHelmConfigs(baseFolder string, helmFolders []string) []models.HelmConfig {
	result := []models.HelmConfig{}
	for _, v := range helmFolders {
		hc := models.HelmConfig{
			Name: v,
			Path: v,
		}
		possibleValuesFile := filepath.Join(baseFolder, hc.Path, "values.yaml")
		if _, err := os.Stat(possibleValuesFile); errors.Is(err, os.ErrNotExist) {
			// if default values file does not exists, use a empty map as values
			hc.Values = map[string]interface{}{}
		} else {
			hc.ValuesFile = filepath.Join(".", hc.Path, "values.yaml")
		}
		result = append(result, hc)
	}
	return result
}

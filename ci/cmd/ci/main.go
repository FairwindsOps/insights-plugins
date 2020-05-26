package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fairwindsops/insights-plugins/trivy/pkg/image"
	"github.com/fairwindsops/insights-plugins/trivy/pkg/models"
	"github.com/thoas/go-funk"
	"gopkg.in/yaml.v3"
)

func main() {
	// TODO Parse out config?
	const configFolder = "./temp/yaml"
	const imageFolder = "./temp/images"
	// TODO Scan YAML, find all images/kind/etc
	err := filepath.Walk(configFolder, getConfig)
	// Untar images, read manifest.json/RepoTags, match tags to YAML
	// Download missing images
	// Scan Images with Trivy

	reportByRef := funk.Map(images, func(image models.Image) (string, []models.VulnerabilityList) {
		results, _ := util.ScanImageFile(image.PullRef)
		return image.PullRef, results
	})
	allReports := image.ConvertTrivyResultsToImageReport(images, reportByRef)
	// Collate results
	finalReport := image.Minimize(allReports, models.MinimizedReport{Images: make([]models.ImageDetailsWithRefs, 0), Vulnerabilities: map[string]models.VulnerabilityDetails{}})

	// Scan with Polaris
	// Send Results up
}

func getConfig(path string, info os.FileInfo, err error) error {
	if strings.HasSuffix()(info.Name(), ".yaml") {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		decoder := yaml.NewDecoder(file)
		for {
			yamlNode := make(map[string]interface{})

			err = decoder.Decode(&yamlNode)
			if err != nil {
				if err != io.EOF {
					return err
				} else {
					break
				}
			}
			kind := yamlNode["kind"].(string)
			if kind == "list" {
				nodes := yamlNode["items"].([]map[string]interface{})
				for _, node := range nodes {
					processYamlNode(node)
				}
			} else {
				processYamlNode(yamlNode)
			}

		}

	}
	return nil
}

func processYamlNode(yamlNode map[string]interface{}) {
owner := models.Resource {
				Kind: yamlNode["kind"].(string),
				Name: yamlNode["metadata"].(map[string]interface{})["name"].(string)
			}
}

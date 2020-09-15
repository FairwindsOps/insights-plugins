package opa

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/fairwindsops/insights-plugins/opa/pkg/opa"
	"github.com/thoas/go-funk"
	"gopkg.in/yaml.v3"

	"github.com/fairwindsops/insights-plugins/ci/pkg/models"
)

const opaVersion = "0.2.8"

// ProcessOPA runs all checks against the provided Custom Check
func ProcessOPA(ctx context.Context, configurationObject models.Configuration, instances []opa.CheckSetting, checks []opa.OPACustomCheck) (models.ReportInfo, error) {
	report := models.ReportInfo{
		Report:   "opa",
		Filename: "opa.json",
		Version:  opaVersion,
	}
	actionItems := make([]opa.ActionItem, 0)
	configFolder := configurationObject.Options.TempFolder + "/configuration/"
	err := filepath.Walk(configFolder, func(path string, info os.FileInfo, err error) error {
		if !strings.HasSuffix(info.Name(), ".yaml") {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		decoder := yaml.NewDecoder(file)
		for {
			yamlNode := map[string]interface{}{}

			err = decoder.Decode(&yamlNode)
			if err != nil {
				if err != io.EOF {
					return err
				}
				break

			}
			resourceKind := yamlNode["kind"].(string)
			if resourceKind == "list" {
				nodes := yamlNode["items"].([]interface{})
				for _, node := range nodes {
					nodeMap := node.(map[string]interface{})
					metadata := nodeMap["metadata"].(map[string]interface{})
					namespace := ""
					if namespaceObj, ok := metadata["namespace"]; ok {
						namespace = namespaceObj.(string)
					}
					resourceName := metadata["name"].(string)
					apiVersion := nodeMap["apiVersion"].(string)
					apiGroup := strings.Split(apiVersion, "/")[0]
					kind := nodeMap["kind"].(string)
					newActionItems, err := processObject(ctx, nodeMap, resourceName, kind, apiGroup, namespace, instances, checks)
					if err != nil {
						return err
					}
					actionItems = append(actionItems, newActionItems...)
				}
			} else {
				metadata := yamlNode["metadata"].(map[string]interface{})
				namespace := ""
				if namespaceObj, ok := metadata["namespace"]; ok {
					namespace = namespaceObj.(string)
				}
				resourceName := metadata["name"].(string)
				apiVersion := yamlNode["apiVersion"].(string)
				apiGroup := strings.Split(apiVersion, "/")[0]
				newActionItems, err := processObject(ctx, yamlNode, resourceName, resourceKind, apiGroup, namespace, instances, checks)
				if err != nil {
					return err
				}
				actionItems = append(actionItems, newActionItems...)
			}
		}
		return nil
	})
	results := map[string]interface{}{
		"ActionItems": actionItems,
	}
	bytes, err := json.Marshal(results)
	if err != nil {
		return report, err
	}
	err = ioutil.WriteFile(configurationObject.Options.TempFolder+"/"+report.Filename, bytes, 0644)
	if err != nil {
		return report, err
	}
	return report, nil
}

func processObject(ctx context.Context, obj map[string]interface{}, resourceName, resourceKind, apiGroup, resourceNamespace string, instances []opa.CheckSetting, checks []opa.OPACustomCheck) ([]opa.ActionItem, error) {
	actionItems := make([]opa.ActionItem, 0)

	for _, instanceObject := range instances {
		instance := instanceObject.GetCustomCheckInstance()
		found := instance.MatchesTarget(apiGroup, resourceKind)
		if !found {
			continue
		}
		maybeCheckObject := funk.Find(checks, func(c opa.OPACustomCheck) bool {
			return c.Name == instance.Spec.CustomCheckName
		})
		if maybeCheckObject == nil {
			continue
		}
		checkObject := maybeCheckObject.(opa.OPACustomCheck)
		check := checkObject.GetCustomCheck()
		newActionItems, err := opa.ProcessCheckForItem(ctx, check, instance, obj, resourceName, resourceKind, resourceNamespace)
		if err != nil {
			return actionItems, err
		}
		actionItems = append(actionItems, newActionItems...)

	}
	return actionItems, nil
}

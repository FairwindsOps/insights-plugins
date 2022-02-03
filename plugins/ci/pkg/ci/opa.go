package ci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fairwindsops/insights-plugins/opa/pkg/kube"
	"github.com/fairwindsops/insights-plugins/opa/pkg/opa"
	"github.com/sirupsen/logrus"
	"github.com/thoas/go-funk"
	"gopkg.in/yaml.v3"

	"github.com/fairwindsops/insights-plugins/ci/pkg/models"
	"github.com/fairwindsops/insights-plugins/ci/pkg/util"
)

const opaVersion = "0.2.8"

// ProcessOPA runs all checks against the provided Custom Check
func (ci CI) ProcessOPA(ctx context.Context) (models.ReportInfo, error) {
	report := models.ReportInfo{
		Report:   "opa",
		Filename: "opa.json",
		Version:  opaVersion,
	}

	instances, checks, err := refreshChecks(*ci.config)
	if err != nil {
		return report, err
	}
	var files []map[string]interface{}
	actionItems := make([]opa.ActionItem, 0)
	configFolder := ci.config.Options.TempFolder + "/configuration/"
	err = filepath.Walk(configFolder, func(path string, info os.FileInfo, err error) error {
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
					files = append(files, nodeMap)
				}
			} else {
				files = append(files, yamlNode)
			}
		}
		return nil
	})
	if err != nil {
		logrus.Warn("Unable to walk through configFolder tree")
		return report, err
	}

	kube.SetFileClient(files)
	for _, nodeMap := range files {
		apiVersion, resourceKind, resourceName, namespace := util.ExtractMetadata(nodeMap)
		apiGroup := strings.Split(apiVersion, "/")[0]
		newActionItems, err := processObject(ctx, nodeMap, resourceName, resourceKind, apiGroup, namespace, instances, checks)
		if err != nil {
			return report, err
		}
		actionItems = append(actionItems, newActionItems...)
	}
	results := map[string]interface{}{
		"ActionItems": actionItems,
	}
	bytes, err := json.Marshal(results)
	if err != nil {
		return report, err
	}
	err = ioutil.WriteFile(filepath.Join(ci.config.Options.TempFolder, report.Filename), bytes, 0644)
	if err != nil {
		return report, err
	}
	return report, nil
}

type opaChecks struct {
	Checks    []opa.OPACustomCheck
	Instances []opa.CheckSetting
}

func refreshChecks(configurationObject models.Configuration) ([]opa.CheckSetting, []opa.OPACustomCheck, error) {
	url := fmt.Sprintf("%s/v0/organizations/%s/ci/opa", configurationObject.Options.Hostname, configurationObject.Options.Organization)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logrus.Warn("Unable to create Request to retrieve checks")
		return nil, nil, err
	}
	token := strings.TrimSpace(os.Getenv("FAIRWINDS_TOKEN"))
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logrus.Warnf("Unable to Get Checks from Insights(%s)", configurationObject.Options.Hostname)
		return nil, nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		logrus.Warnf("Unable to Get Checks from Insights(%s)", configurationObject.Options.Hostname)
		if err != nil {
			logrus.Warn("Unable to read response body")
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("Insights returned unexpected HTTP status code: %d - %v", resp.StatusCode, string(body))
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Warn("Unable to read results")
		return nil, nil, err
	}
	var checkBody opaChecks
	err = json.Unmarshal(body, &checkBody)
	if err != nil {
		logrus.Warn("Unable to unmarshal results")
		return nil, nil, err
	}
	return checkBody.Instances, checkBody.Checks, nil
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
		check := maybeCheckObject.(opa.OPACustomCheck)
		newActionItems, err := opa.ProcessCheckForItem(ctx, check, instance, obj, resourceName, resourceKind, resourceNamespace)
		if err != nil {
			return actionItems, err
		}
		actionItems = append(actionItems, newActionItems...)

	}
	return actionItems, nil
}

func (ci *CI) OPAEnabled() bool {
	return *ci.config.Reports.OPA.Enabled
}

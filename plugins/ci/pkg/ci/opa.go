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

	opaversion "github.com/fairwindsops/insights-plugins/plugins/opa"
	"github.com/fairwindsops/insights-plugins/plugins/opa/pkg/kube"
	"github.com/fairwindsops/insights-plugins/plugins/opa/pkg/opa"
	"github.com/fairwindsops/insights-plugins/plugins/opa/pkg/rego"
	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/util"
)

// ProcessOPA runs all checks against the provided Custom Check
func (ci CIScan) ProcessOPA(ctx context.Context) (models.ReportInfo, error) {
	report := models.ReportInfo{
		Report:   "opa",
		Filename: "opa.json",
		Version:  opaversion.String(),
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
	var allErrs error = nil
	for _, check := range checks {
		logrus.Debugf("Check %s is version %.1f\n", check.Name, check.Version)
		switch check.Version {
		case 1.0:
			for _, instanceObject := range instances {
				if instanceObject.CheckName != check.Name {
					continue
				}
				logrus.Debugf("Found instance %s to match check %s", instanceObject.AdditionalData.Name, check.Name)
				instance := instanceObject.GetCustomCheckInstance()
				foundTargetInInstance := instance.MatchesTarget(apiGroup, resourceKind)
				if !foundTargetInInstance {
					logrus.Debugf("No Kubernetes target matches for APIGroup %s and resource %s in check %s / instance %s targets: %v\n", apiGroup, resourceKind, check.Name, instanceObject.AdditionalData.Name, instance.Spec.Targets)
					continue
				}
				newActionItems, err := opa.ProcessCheckForItem(ctx, check, instance, obj, resourceName, resourceKind, resourceNamespace, &rego.InsightsInfo{InsightsContext: "CI/CD"})
				if err != nil {
					allErrs = multierror.Append(allErrs, fmt.Errorf("error while processing check %s / instance %s: %v", check.Name, instanceObject.AdditionalData.Name, err))
				}
				actionItems = append(actionItems, newActionItems...)
			}
		case 2.0:
			newActionItems, err := opa.ProcessCheckForItemV2(ctx, check, obj, resourceName, resourceKind, resourceNamespace, &rego.InsightsInfo{InsightsContext: "CI/CD"})
			if err != nil {
				allErrs = multierror.Append(allErrs, fmt.Errorf("error while processing check %s: %v", check.Name, err))
			}
			actionItems = append(actionItems, newActionItems...)
		default:
			allErrs = multierror.Append(allErrs, fmt.Errorf("CustomCheck %s is an unexpected version %.1f and will not be run - this could cause CI to be blocked", check.Name, check.Version))
		}
	}
	return actionItems, allErrs
}

func (ci *CIScan) OPAEnabled() bool {
	return *ci.config.Reports.OPA.Enabled
}

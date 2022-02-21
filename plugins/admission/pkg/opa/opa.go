package opa

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fairwindsops/insights-plugins/opa/pkg/opa"
	"github.com/fairwindsops/insights-plugins/opa/pkg/rego"
	"github.com/thoas/go-funk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fairwindsops/insights-plugins/admission/pkg/models"
)

const opaVersion = "0.2.8"

// ProcessOPA runs all checks against the provided Custom Check
func ProcessOPA(ctx context.Context, obj map[string]interface{}, resourceName, apiGroup, resourceKind, resourceNamespace string, configuration models.Configuration) (models.ReportInfo, error) {
	report := models.ReportInfo{
		Report:  "opa",
		Version: opaVersion,
	}
	actionItems := make([]opa.ActionItem, 0)
	cluster := os.Getenv("FAIRWINDS_CLUSTER")
	for _, check := range configuration.OPA.CustomChecks {
		logrus.Debugf("Check %s is version %.1f\n", check.Name, check.Version)
		switch check.Version {
		case 1.0:
			for _, instanceObject := range configuration.OPA.CustomCheckInstances {
				if instanceObject.CheckName != check.Name {
					continue
				}
				logrus.Debugf("Found instance %s to match check %s", instanceObject.CheckName, check.Name)
				instance := opa.CustomCheckInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name: instanceObject.AdditionalData.Name,
					},
					Spec: opa.CustomCheckInstanceSpec{
						CustomCheckName: instanceObject.CheckName,
						Output:          instanceObject.AdditionalData.Output,
						Parameters:      instanceObject.AdditionalData.Parameters,
						Targets: funk.Map(instanceObject.Targets, func(s string) opa.KubeTarget {
							splitValues := strings.Split(s, "/")
							return opa.KubeTarget{
								APIGroups: []string{splitValues[0]},
								Kinds:     []string{splitValues[1]},
							}
						}).([]opa.KubeTarget),
					},
				}
				foundTargetInInstance := false
				for _, target := range instance.Spec.Targets {
					if apiGroup == target.APIGroups[0] && resourceKind == target.Kinds[0] {
						foundTargetInInstance = true
					}
				}
				if !foundTargetInInstance {
					continue
				}
				newActionItems, err := opa.ProcessCheckForItem(ctx, check, instance, obj, resourceName, resourceKind, resourceNamespace, &rego.InsightsInfo{InsightsContext: "AdmissionController", "Cluster": cluster})
				if err != nil {
					return report, err
				}
				actionItems = append(actionItems, newActionItems...)
			}
		case 2.0:
			newActionItems, err := opa.ProcessCheckForItemV2(ctx, check, obj, resourceName, resourceKind, resourceNamespace, &rego.InsightsInfo{InsightsContext: "AdmissionController", Cluster: cluster})
			if err != nil {
				return report, err
			}
			actionItems = append(actionItems, newActionItems...)
		default:
			return report, fmt.Errorf("CustomCheck %s is an unexpected version %.1f and will not be run - this could cause admission control to be blocked", check.Name, check.Version)
		} // switch
	} // iterate checks
	results := map[string]interface{}{
		"ActionItems": actionItems,
	}
	bytes, err := json.Marshal(results)
	if err != nil {
		return report, err
	}
	report.Contents = bytes
	return report, nil
}

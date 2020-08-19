package opa

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/fairwindsops/insights-plugins/opa/pkg/opa"
	"github.com/thoas/go-funk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fairwindsops/insights-plugins/admission/pkg/models"
)

// ProcessOPA runs all checks against the provided Custom Check
func ProcessOPA(ctx context.Context, obj map[string]interface{}, resourceName, apiGroup, resourceKind, resourceNamespace string, configuration models.Configuration) (models.ReportInfo, error) {
	report := models.ReportInfo{
		Report: "opa",
	}
	actionItems := make([]opa.ActionItem, 0)
	for _, instanceObject := range configuration.OPA.CustomCheckInstances {
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
		found := false
		for _, target := range instance.Spec.Targets {
			if apiGroup == target.APIGroups[0] && resourceKind == target.Kinds[0] {
				found = true
			}
		}
		if !found {
			continue
		}
		maybeCheckObject := funk.Find(configuration.OPA.CustomChecks, func(c opa.OPACustomCheck) bool {
			return c.Name == instance.Spec.CustomCheckName
		})
		if maybeCheckObject == nil {
			continue
		}
		checkObject := maybeCheckObject.(opa.OPACustomCheck)
		check := opa.CustomCheck{
			ObjectMeta: metav1.ObjectMeta{
				Name: checkObject.Name,
			},
			Spec: opa.CustomCheckSpec{
				Output: opa.OutputFormat{
					Title:       checkObject.Title,
					Severity:    checkObject.Severity,
					Remediation: checkObject.Remediation,
					Category:    checkObject.Category,
				},
				Rego: checkObject.Rego,
			},
		}
		newActionItems, err := opa.ProcessCheckForItem(ctx, check, instance, obj, resourceName, resourceKind, resourceNamespace)
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
	report.Contents = bytes
	return report, nil
}

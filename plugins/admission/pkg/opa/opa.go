package opa

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/opa/pkg/opa"
	"github.com/fairwindsops/insights-plugins/plugins/opa/pkg/rego"
	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
	"github.com/thoas/go-funk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/models"
)

const opaVersion = "2.0.0"

// ProcessOPA runs all CustomChecks against the provided Kubernetes object.
func ProcessOPA(ctx context.Context, obj map[string]interface{}, resourceName, apiGroup, resourceKind, resourceNamespace string, configuration models.Configuration) (models.ReportInfo, error) {
	report := models.ReportInfo{
		Report:  "opa",
		Version: opaVersion,
	}
	actionItems := make([]opa.ActionItem, 0)
	var allErrs error = nil
	cluster := os.Getenv("FAIRWINDS_CLUSTER")
	for _, check := range configuration.OPA.CustomChecks {
		logrus.Debugf("Check %s is version %.1f\n", check.Name, check.Version)
		switch check.Version {
		case 1.0:
			newActionItems, err := ProcessOPAV1(ctx, obj, resourceName, apiGroup, resourceKind, resourceNamespace, check, configuration.OPA.CustomCheckInstances, rego.InsightsInfo{InsightsContext: "AdmissionController", Cluster: cluster})
			actionItems = append(actionItems, newActionItems...)
			if err != nil {
				allErrs = multierror.Append(allErrs, err)
			}
		case 2.0:
			newActionItems, err := ProcessOPAV2(ctx, obj, resourceName, apiGroup, resourceKind, resourceNamespace, check, rego.InsightsInfo{InsightsContext: "AdmissionController", Cluster: cluster})
			actionItems = append(actionItems, newActionItems...)
			if err != nil {
				allErrs = multierror.Append(allErrs, err)
			}
		default:
			allErrs = multierror.Append(allErrs, fmt.Errorf("CustomCheck %s is an unexpected version %.1f and will not be run - this could cause admission control to be blocked", check.Name, check.Version))
		}
	}
	results := map[string]interface{}{
		"ActionItems": actionItems,
	}
	bytes, err := json.Marshal(results)
	if err != nil {
		return report, err
	}
	report.Contents = bytes
	return report, allErrs
}

// ProcessOPAV1 runs a V1 CustomCheck against a Kubernetes object,
// returning action items and potentially multiple wrapped errors (as returned
// by multiple instances; CheckSettings associated with a CustomCheck).
func ProcessOPAV1(ctx context.Context, obj map[string]interface{}, resourceName, apiGroup, resourceKind, resourceNamespace string, check opa.OPACustomCheck, checkInstances []opa.CheckSetting, insightsInfo rego.InsightsInfo) ([]opa.ActionItem, error) {
	actionItems := make([]opa.ActionItem, 0)
	var allErrs error = nil
	for _, instanceObject := range checkInstances {
		if instanceObject.CheckName != check.Name {
			continue
		}
		logrus.Debugf("Found instance %s to match check %s", instanceObject.AdditionalData.Name, check.Name)
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
		newActionItems, err := opa.ProcessCheckForItem(ctx, check, instance, obj, resourceName, resourceKind, resourceNamespace, &insightsInfo)
		if err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("error while processing check %s / instance %s: %v", check.Name, instanceObject.AdditionalData.Name, err))
		}
		actionItems = append(actionItems, newActionItems...)
	}
	return actionItems, allErrs
}

// ProcessOPAV2 runs a V2 CustomCheck against a Kubernetes object,
// returning action items and any error encountered while processing the
// check.
func ProcessOPAV2(ctx context.Context, obj map[string]interface{}, resourceName, apiGroup, resourceKind, resourceNamespace string, check opa.OPACustomCheck, insightsInfo rego.InsightsInfo) ([]opa.ActionItem, error) {
	newActionItems, err := opa.ProcessCheckForItemV2(ctx, check, obj, resourceName, resourceKind, resourceNamespace, &insightsInfo)
	return newActionItems, err
}

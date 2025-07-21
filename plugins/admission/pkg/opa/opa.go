package opa

import (
	"context"
	"encoding/json"

	opaVersion "github.com/fairwindsops/insights-plugins/plugins/opa"
	"github.com/fairwindsops/insights-plugins/plugins/opa/pkg/opa"
	"github.com/fairwindsops/insights-plugins/plugins/opa/pkg/rego"
	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/models"
)

// ProcessOPA runs all CustomChecks against the provided Kubernetes object.
func ProcessOPA(ctx context.Context, obj map[string]any, req admission.Request, configuration models.Configuration, iConfig models.InsightsConfig) (models.ReportInfo, error) {
	report := models.ReportInfo{
		Report:  "opa",
		Version: opaVersion.String(),
	}
	actionItems := make([]opa.ActionItem, 0)
	var allErrs error = nil
	requestInfo := rego.InsightsInfo{InsightsContext: "AdmissionController", Cluster: iConfig.Cluster, AdmissionRequest: &req}

	opaCustomChecks, opaCustomLibsV0, opaCustomLibsV1 := opa.GetOPACustomChecksAndLibraries(configuration.OPA.CustomChecks)
	logrus.Infof("Found %d checks, %d instances, %d libs V0 and %d libs V1", len(opaCustomChecks), len(configuration.OPA.CustomCheckInstances), len(opaCustomLibsV0), len(opaCustomLibsV1))
	anyChekIsV1 := false
	for _, check := range opaCustomChecks {
		newActionItems, err := ProcessOPAV2(ctx, obj, req.AdmissionRequest.Name, req.AdmissionRequest.RequestKind.Group, req.AdmissionRequest.RequestKind.Kind, req.AdmissionRequest.Namespace, check, opaCustomLibsV0, opaCustomLibsV1, &requestInfo)
		actionItems = append(actionItems, newActionItems...)
		if err != nil {
			allErrs = multierror.Append(allErrs, err)
		}
	}
	if anyChekIsV1 {
		logrus.Info("OPA v1 will be deprecated after Mar 31, 2025. Visit: https://insights.docs.fairwinds.com/features/insights-cli/#opa-v1-deprecation for more information.")
	}
	results := map[string]any{
		"ActionItems": actionItems,
	}
	bytes, err := json.Marshal(results)
	if err != nil {
		return report, err
	}
	report.Contents = bytes
	return report, allErrs
}

// ProcessOPAV2 runs a V2 CustomCheck against a Kubernetes object,
// returning action items and any error encountered while processing the
// check.
func ProcessOPAV2(ctx context.Context, obj map[string]any, resourceName, apiGroup, resourceKind, resourceNamespace string, check opa.OPACustomCheck, opaCustomLibsV0, opaCustomLibsV1 []opa.OPACustomLibrary, insightsInfo *rego.InsightsInfo) ([]opa.ActionItem, error) {
	newActionItems, err := opa.ProcessCheckForItemV2(ctx, check, obj, resourceName, resourceKind, resourceNamespace, opaCustomLibsV0, opaCustomLibsV1, insightsInfo)
	return newActionItems, err
}

package polaris

import (
	"context"
	"encoding/json"

	polarisconfiguration "github.com/fairwindsops/polaris/pkg/config"
	"github.com/fairwindsops/polaris/pkg/kube"
	"github.com/fairwindsops/polaris/pkg/validator"
	fwebhook "github.com/fairwindsops/polaris/pkg/webhook"

	"github.com/fairwindsops/insights-plugins/admission/pkg/models"
)

// GetPolarisReport returns the polaris report for the provided manifest.
func GetPolarisReport(ctx context.Context, config polarisconfiguration.Configuration, kind string, manifest []byte) (models.ReportInfo, error) {
	report := models.ReportInfo{
		Report: "polaris",
	}
	// Scan with Polaris
	pod, originalObject, err := fwebhook.GetObjectFromRawRequest(manifest)
	if err != nil {
		return report, err
	}
	controller, err := kube.NewGenericWorkloadFromPod(pod, originalObject)
	if err != nil {
		return report, err
	}
	controller.Kind = kind
	controllerResult, err := validator.ValidateController(ctx, &config, controller)
	if err != nil {
		return report, err
	}

	report.Version = validator.PolarisOutputVersion
	auditData := validator.AuditData{
		PolarisOutputVersion: validator.PolarisOutputVersion,
		Results:              []validator.ControllerResult{controllerResult},
	}
	bytes, err := json.Marshal(auditData)
	if err != nil {
		return report, err
	}
	report.Contents = bytes
	return report, nil
}

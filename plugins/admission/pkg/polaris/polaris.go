package polaris

import (
	"context"
	"encoding/json"

	polarisconfiguration "github.com/fairwindsops/polaris/pkg/config"
	fwkube "github.com/fairwindsops/polaris/pkg/kube"
	"github.com/fairwindsops/polaris/pkg/validator"

	"github.com/fairwindsops/insights-plugins/admission/pkg/models"
)

// GetPolarisReport returns the polaris report for the provided manifest.
func GetPolarisReport(ctx context.Context, config polarisconfiguration.Configuration, manifest []byte) (models.ReportInfo, error) {
	report := models.ReportInfo{
		Report: "polaris",
	}
	// Scan with Polaris
	controller, err := fwkube.NewGenericResourceFromBytes(manifest)
	if err != nil {
		return report, err
	}
	controllerResult, err := validator.ApplyAllSchemaChecks(&config, nil, controller)
	if err != nil {
		return report, err
	}

	report.Version = validator.PolarisOutputVersion
	auditData := validator.AuditData{
		PolarisOutputVersion: validator.PolarisOutputVersion,
		Results:              []validator.Result{controllerResult},
	}
	bytes, err := json.Marshal(auditData)
	if err != nil {
		return report, err
	}
	report.Contents = bytes
	return report, nil
}

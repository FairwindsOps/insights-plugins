package ci

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
)

const workloadsReportVersion = "0.1.0"

func (ci *CIScan) GetWorkloadReport(resources []models.Resource) (*models.ReportInfo, error) {
	workloadsReport := models.ReportInfo{
		Report:   "scan-workloads",
		Filename: "scan-workloads.json",
	}
	resourceBytes, err := json.Marshal(map[string]interface{}{"Resources": resources})
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(filepath.Join(ci.config.Options.TempFolder, workloadsReport.Filename), resourceBytes, 0644)
	if err != nil {
		return nil, err
	}

	workloadsReport.Version = workloadsReportVersion
	return &workloadsReport, nil
}

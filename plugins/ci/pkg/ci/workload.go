package ci

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
)

const workloadsReportVersion = "0.2.0"

func (ci *CIScan) GetWorkloadReport(resources []models.Resource) (*models.ReportInfo, error) {
	workloadsReport := models.ReportInfo{
		Report:   "scan-workloads",
		Filename: "scan-workloads.json",
		Version:  workloadsReportVersion,
	}
	resourceBytes, err := json.Marshal(map[string]interface{}{"Resources": resources})
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(filepath.Join(ci.config.Options.TempFolder, workloadsReport.Filename), resourceBytes, 0644)
	if err != nil {
		return nil, err
	}
	return &workloadsReport, nil
}

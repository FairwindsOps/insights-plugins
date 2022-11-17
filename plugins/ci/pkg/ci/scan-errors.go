package ci

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
)

// processScanErrorsReportProperties accepts a ScanErrorsReportProperties and returns
// them inside of a more generic ReportInfo type.
// This prepares a ScanErrorsReport to be sent to Insights with other reports.
func (ci *CIScan) processScanErrorsReportProperties(reportProperties models.ScanErrorsReportProperties) (models.ReportInfo, error) {
	report := models.ReportInfo{
		Report:   "scan-errors",
		Version:  models.ScanErrorsReportVersion,
		Filename: "scan-errors.json",
	}
	// This report is also (somewhat unnecessarily) written to a file, so that
	// CIScan.sendResults() can process its data along with other report types.
	file, err := json.MarshalIndent(reportProperties, "", " ")
	if err != nil {
		return report, fmt.Errorf("while encoding report output: %w", err)
	}
	err = os.WriteFile(filepath.Join(ci.config.Options.TempFolder, report.Filename), file, 0644)
	if err != nil {
		return report, fmt.Errorf("while writing report output: %w", err)
	}
	return report, nil
}

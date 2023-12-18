package ci

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
)

// processScanErrorsReportProperties prepares a ScanErrorsReport to be sent to
// Insights with other reports, by writing the report to a file to be later
// read and processed in the same way as other reports.
func (ci *CIScan) processScanErrorsReportProperties(reportProperties models.ScanErrorsReportProperties) (*models.ReportInfo, error) {
	report := &models.ReportInfo{
		Report:   "scan-errors",
		Version:  models.ScanErrorsReportVersion,
		Filename: "scan-errors.json",
	}
	// This report is (somewhat unnecessarily) written to a file, so that
	// CIScan.sendResults() can process its data along with other report types.
	file, err := json.MarshalIndent(reportProperties, "", " ")
	if err != nil {
		return nil, fmt.Errorf("while encoding report output: %w", err)
	}
	err = os.WriteFile(filepath.Join(ci.config.Options.TempFolder, report.Filename), file, 0644)
	if err != nil {
		return nil, fmt.Errorf("while writing report output: %w", err)
	}
	return report, nil
}

func createErrorItemMessage(r models.ScanErrorsReportResult) string {
	var sb strings.Builder
	if r.ErrorContext != "" {
		sb.WriteString(r.ErrorContext)
		sb.WriteString("/")
	}
	if r.Kind != "" {
		sb.WriteString(r.Kind)
		sb.WriteString("/")
	}
	if r.ResourceName != "" {
		sb.WriteString(r.ResourceName)
	}
	if sb.Len() > 0 {
		sb.WriteString(" - ")
	}
	sb.WriteString(r.ErrorMessage)
	if r.Filename != "" {
		sb.WriteString(" (")
		sb.WriteString(r.Filename)
		sb.WriteString(")")
	}
	return sb.String()
}

package models

const ScanErrorsReportVersion = "0.0.1"

// ScanErrorResult contains a single error encountered during a scan.
// This satisfies the GO Error interface, and provides additional error context
// to be bubbled up into a scan-errors report.
type ScanErrorsReportResult struct {
	Kind         string  `json:"kind"`
	ResourceName string  `json:"resourceName"`
	ErrorMessage string  `json:"errorMessage"` // error message returned during a scan
	ErrorContext string  `json:"errorContext"` // where were we / what was happening when the error occurred
	Filename     string  // filename being scanned that relates to this error
	Remediation  string  `json:"remediation"`
	Severity     float64 `json:"severity"`
	Category     string  `json:"category"`
}

// The Error receiver satisfies the Go error interface, allowing the
// ScanErrorsReportResult type to be passed as an error, and type-casted back
// to a ScanErrorsReportResult type for inclusion in the ScanErrorsReport.
func (r ScanErrorsReportResult) Error() string {
	return r.ErrorMessage
}

// ScanErrorsReportProperties contains multiple ScanErrorsReportResults.
type ScanErrorsReportProperties struct {
	Items []ScanErrorsReportResult `json:"results"`
}

// ScanErrorsReport contains ScanErrorReportProperties and the report version.
type ScanErrorsReport struct {
	Version string
	Report  ScanErrorsReportProperties
}

// AddItemFromError type-asserts an Error type into a ScanErrorsReportResult
// type, and adds it to the slice stored in the ScanErrorsReportProperties
// receiver.
func (reportProperties *ScanErrorsReportProperties) AddItemFromError(e error) {
	newItem, ok := e.(ScanErrorsReportResult)
	if !ok {
		// maybe add an item without context or ResourceName?
		return
	}
	reportProperties.Items = append(reportProperties.Items, newItem)
}

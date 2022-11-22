package models

import (
	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
)

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

// AddScanErrorsReportResultFromError type-asserts an Error type into a ScanErrorsReportResult
// type, and adds it to the slice stored in the ScanErrorsReportProperties
// receiver.
func (reportProperties *ScanErrorsReportProperties) AddScanErrorsReportResultFromError(e error) {
	var newItem ScanErrorsReportResult
	switch v := e.(type) {
	case *multierror.Error: // multiple results
		for _, singleErr := range v.Errors {
			newItem, ok := singleErr.(ScanErrorsReportResult)
			if !ok {
				newItem = NewScanErrorsReportResultWithoutContext(singleErr)
			}
			reportProperties.Items = append(reportProperties.Items, newItem)
		}
	case ScanErrorsReportResult: // A single result
		reportProperties.Items = append(reportProperties.Items, v)
	default:
		newItem = NewScanErrorsReportResultWithoutContext(e)
		reportProperties.Items = append(reportProperties.Items, newItem)
	}
	/*
	   multipleErrs, ok := e.(*multierror.Error)
	   	if !ok {
	   		newItem, isOurErrorType := e.(ScanErrorsReportResult)
	   		if !isOurErrorType {
	   			// marker
	   			return
	   		}
	   		reportProperties.Items = append(reportProperties.Items, newItem)
	   		return
	   	}
	   	for _, singleErr := range multipleErrs.Errors {
	   		newItem, ok := singleErr.(ScanErrorsReportResult)
	   		if !ok {
	   			// maybe add an item without context or ResourceName?
	   			return
	   		}
	   		reportProperties.Items = append(reportProperties.Items, newItem)
	   	}
	*/
	return
}

// NewScanErrorsReportResultWithoutContext accepts an error interface that is NOT
// our type ScanErrorsReportResult, and logs that there is insufficient
// context for this error, while returning a ScanErrorsReportResult type with
// required fields populated (even if inadiquitly).
func NewScanErrorsReportResultWithoutContext(e error) ScanErrorsReportResult {
	logrus.Warnf("adding this error to the scan-errors report which does not have sufficient context, please return a ScanErrorsReportResult instead of a standard error: %v", e)
	r := ScanErrorsReportResult{
		Kind:         "ErrorWithoutContext",
		ResourceName: "unknown",
		ErrorContext: "performing an action in CI",
		ErrorMessage: e.Error(),
	}
	return r
}

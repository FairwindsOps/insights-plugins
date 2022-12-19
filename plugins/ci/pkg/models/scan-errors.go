package models

import (
	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
)

const (
	ScanErrorsReportVersion             = "0.0.1"
	ScanErrorsReportDefaultKind         = "CIErrorWithoutContext"
	ScanErrorsReportDefaultResourceName = "unknown"
	ScanErrorsReportDefaultErrorContext = "performing an action in CI"
)

// ScanErrorResult contains a single error encountered during a scan.
// This satisfies the GO Error interface, and provides additional error context
// to be bubbled up into a scan-errors report.
type ScanErrorsReportResult struct {
	// IF adding a field to this struct, also update the FillUnsetFields receiver!
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

// FillUnsetFields populates any unset ScanErrorsReportResult fields with
// those from the parameter.
// This is useful to provide context only when an upstream error does not
// already contain any.
func (r *ScanErrorsReportResult) FillUnsetFields(f ScanErrorsReportResult) {
	orig := *r
	var anyChanges bool
	if r.Kind == "" {
		anyChanges = true
		r.Kind = f.Kind
	}
	if r.ResourceName == "" {
		anyChanges = true
		r.ResourceName = f.ResourceName
	}
	if r.ErrorContext == "" {
		anyChanges = true
		r.ErrorContext = f.ErrorContext
	}
	if r.Filename == "" {
		anyChanges = true
		r.Filename = f.Filename
	}
	if r.Remediation == "" {
		anyChanges = true
		r.Remediation = f.Remediation
	}
	if r.Severity == 0.0 {
		anyChanges = true
		r.Severity = f.Severity
	}
	if r.Category == "" {
		anyChanges = true
		r.Category = f.Category
	}
	if anyChanges {
		logrus.Debugf("updated missing fields in %#v, using values from %#v, and final result is: %#v", orig, f, *r)
	}
}

// FillUnsetRequiredFieldsWithDefaults populates any unset
// ScanErrorsReportResult fields that are required, with defaults.
func (r *ScanErrorsReportResult) FillUnsetRequiredFieldsWithDefaults() {
	if r.Kind == "" {
		logrus.Warnf("setting required field Kind to %q for this ScanErrorsReportResult: %#v", ScanErrorsReportDefaultKind, *r)
		r.Kind = ScanErrorsReportDefaultKind
	}
	if r.ResourceName == "" {
		logrus.Warnf("setting required field ResourceName to %q for this ScanErrorsReportResult: %#v", ScanErrorsReportDefaultResourceName, *r)
		r.ResourceName = ScanErrorsReportDefaultResourceName
	}
	if r.ErrorContext == "" {
		logrus.Warnf("setting required field ErrorContext to %q for this ScanErrorsReportResult: %#v", ScanErrorsReportDefaultErrorContext, *r)
		r.ErrorContext = ScanErrorsReportDefaultErrorContext
	}
	if r.ErrorMessage == "" {
		logrus.Errorf("this ScanError lacks an error message and will likely cause a 500 error if submitted to the API: %#v", *r)
	}
}

// ScanErrorsReportProperties contains multiple ScanErrorsReportResults.
type ScanErrorsReportProperties struct {
	Items []ScanErrorsReportResult `json:"results"`
}

// ScanErrorsReport contains ScanErrorsReportProperties and the report version.
type ScanErrorsReport struct {
	Version string
	Report  ScanErrorsReportProperties
}

// AddScanErrorsReportResultFromError type-asserts an Error type into a ScanErrorsReportResult
// type, and adds it to the slice stored in the ScanErrorsReportProperties
// receiver. Any additional parameters of type ScanErrorsReportResult are used
// only to fill in missing fields of the first error parameter.
// For example: AddScanErrorsReportResultFromError(err, err2) will populate
// any missing fields from err, with values from err2, such as ErrorContext or
// Remediation.
func (reportProperties *ScanErrorsReportProperties) AddScanErrorsReportResultFromError(e error, dataForMissingFields ...ScanErrorsReportResult) {
	logrus.Debugf("processing error for addition to ScanErrorsReport: %#v", e)
	var newItem ScanErrorsReportResult
	switch v := e.(type) {
	case nil:
		return
	case *multierror.Error: // multiple results
		logrus.Debugf("processing a multierror while adding results to ScanErrorsReport: %#v", v)
		for _, singleErr := range v.Errors {
			reportProperties.AddScanErrorsReportResultFromError(singleErr, dataForMissingFields...)
		}
		return
	case ScanErrorsReportResult: // A single result
		newItem = v
	default:
		newItem = ScanErrorsReportResult{
			ErrorMessage: e.Error(),
		}
	}
	for _, d := range dataForMissingFields {
		newItem.FillUnsetFields(d)
	}
	newItem.FillUnsetRequiredFieldsWithDefaults()
	logrus.Debugf("appending this error to ScanErrorsReportProperties: %#v", newItem)
	reportProperties.Items = append(reportProperties.Items, newItem)
	return
}

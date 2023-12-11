package models

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
)

func TestAddScanErrorsReportResultFromAnError(t *testing.T) {
	want := ScanErrorsReportProperties{
		Items: []ScanErrorsReportResult{
			{
				ErrorMessage: "text from a standard error",
				// The following are defaults if required fields are unset.
				ErrorContext: "performing an action in CI",
				Kind:         "CIErrorWithoutContext",
				ResourceName: "unknown",
			},
		},
	}
	got := ScanErrorsReportProperties{}
	got.AddScanErrorsReportResultFromError(fmt.Errorf("text from a standard error"))
	assert.Equal(t, want, got)
}

func TestAddScanErrorsReportResultFromAScanErrorsReportResultWithAnAdditionalResultToAddContext(t *testing.T) {
	want := ScanErrorsReportProperties{
		Items: []ScanErrorsReportResult{
			{
				ErrorMessage: "an error with additional context",
				ErrorContext: "context from the error",
				Kind:         "deployment",
				ResourceName: "myapp",
				Remediation:  "fix the deployment",
				Severity:     2.0,
				Category:     "Security",
				Filename:     "deployment.yaml",
			},
		},
	}
	anError := ScanErrorsReportResult{
		ErrorMessage: "an error with additional context",
		ErrorContext: "context from the error",
	}
	toPopulateMissingFields := ScanErrorsReportResult{
		ErrorMessage: "this will not be used and should not be overwritten from the first result",
		ErrorContext: "this will not be used and should not be overwritten from the first result",
		Kind:         "deployment",
		ResourceName: "myapp",
		Remediation:  "fix the deployment",
		Severity:     2.0,
		Category:     "Security",
		Filename:     "deployment.yaml",
	}
	got := ScanErrorsReportProperties{}
	got.AddScanErrorsReportResultFromError(anError, toPopulateMissingFields)
	assert.Equal(t, want, got)
}

func TestAddScanErrorsReportResultFromAMultiError(t *testing.T) {
	want := ScanErrorsReportProperties{
		Items: []ScanErrorsReportResult{
			{
				ErrorMessage: "an error with additional context",
				ErrorContext: "context from the first error",
				Kind:         "deployment",
				ResourceName: "myapp",
				Remediation:  "fix the deployment",
				Severity:     2.0,
				Category:     "Security",
				Filename:     "deployment.yaml",
			},
			{
				ErrorMessage: "another error with additional context",
				ErrorContext: "context from the second error",
				Kind:         "statefulset",
				ResourceName: "yourapp",
				Remediation:  "fix the statefulset",
				Severity:     2.5,
				Category:     "Security",
				Filename:     "statefulset.yaml",
			},
			{
				ErrorMessage: "a third error with additional context",
				ErrorContext: "a general; fallthrough context", // from toPopulateMissingFields
				Kind:         "CIErrorWithoutContext",          // a default for a required value
				ResourceName: "unknown",                        // a default for a required value
				Remediation:  "fix something",                  // from toPopulateMissingFields
				Severity:     1.5,                              // from toPopulateMissingFields
				Category:     "Reliability",                    // from toPopulateMissingFields
			},
		},
	}
	multipleErrors := new(multierror.Error)
	multipleErrors = multierror.Append(
		ScanErrorsReportResult{
			ErrorMessage: "an error with additional context",
			ErrorContext: "context from the first error",
			Kind:         "deployment",
			ResourceName: "myapp",
			Severity:     2.0,
			Category:     "Security",
			Remediation:  "fix the deployment",
			Filename:     "deployment.yaml",
		},
		ScanErrorsReportResult{
			ErrorMessage: "another error with additional context",
			ErrorContext: "context from the second error",
			Kind:         "statefulset",
			ResourceName: "yourapp",
			Severity:     2.5,
			Category:     "Security",
			Remediation:  "fix the statefulset",
			Filename:     "statefulset.yaml",
		},
		ScanErrorsReportResult{
			ErrorMessage: "a third error with additional context",
		},
	)
	toPopulateMissingFields := ScanErrorsReportResult{
		ErrorMessage: "this will not be used and should not be overwritten",
		ErrorContext: "a general; fallthrough context",
		Remediation:  "fix something",
		Severity:     1.5,
		Category:     "Reliability",
	}
	got := ScanErrorsReportProperties{}
	got.AddScanErrorsReportResultFromError(multipleErrors, toPopulateMissingFields)
	assert.Equal(t, want, got)
}

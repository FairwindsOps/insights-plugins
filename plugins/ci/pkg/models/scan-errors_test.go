package models

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddScanErrorsReportResultFromAnError(t *testing.T) {
	want := ScanErrorsReportProperties{
		Items: []ScanErrorsReportResult{
			ScanErrorsReportResult{
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
			ScanErrorsReportResult{
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

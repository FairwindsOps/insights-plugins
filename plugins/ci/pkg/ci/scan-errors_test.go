package ci

import (
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestAddScanErrorsReportResultFromAMultiError(t *testing.T) {
	assert.Equal(t, "context from the first error/deployment/myapp - an error with additional context (deployment.yaml)",
		createErrorItemMessage(models.ScanErrorsReportResult{
			ErrorMessage: "an error with additional context",
			ErrorContext: "context from the first error",
			Kind:         "deployment",
			ResourceName: "myapp",
			Severity:     2.0,
			Category:     "Security",
			Remediation:  "fix the deployment",
			Filename:     "deployment.yaml",
		}))
}

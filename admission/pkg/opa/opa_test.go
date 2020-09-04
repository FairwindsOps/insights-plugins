package opa

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/fairwindsops/insights-plugins/opa/pkg/opa"
	"github.com/stretchr/testify/assert"

	"github.com/fairwindsops/insights-plugins/admission/pkg/models"
)

// TestProcessOPA runs all checks against the provided Custom Check
func TestProcessOPA(t *testing.T) {
	config := models.Configuration{}
	checks := []opa.OPACustomCheck{}
	instances := []opa.CheckSetting{}
	config.OPA.CustomCheckInstances = instances
	config.OPA.CustomChecks = checks
	report, err := ProcessOPA(context.TODO(), nil, "", "", "", "", config)
	assert.Equal(t, "opa", report.Report)
	assert.NoError(t, err)
	var reportObject map[string]interface{}
	err = json.Unmarshal(report.Contents, &reportObject)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(reportObject["ActionItems"].([]interface{})))
}

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

	object := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labelS": map[string]string{},
		},
	}
	check := opa.OPACustomCheck{
		Name: "check1",
		Rego: `
package fairwinds
labelrequired[results] {
  provided := {label | input.metadata.labels[label]}
  required := {label | label := input.parameters.labels[_]}
  missing := required - provided
  count(missing) > 0
  description := sprintf("Label %v is missing", [missing])
  severity := 0.1 * count(missing)
  results := {
    "description": description,
    "severity": severity,
  }
}
		`,
	}
	checks = append(checks, check)
	instance := opa.CheckSetting{
		CheckName: "check1",
		Targets: []string{
			"/Pod",
		},
	}
	instance.AdditionalData.Name = "check-instance"
	instance.AdditionalData.Parameters = map[string]interface{}{
		"labels": []string{"hr"},
	}
	instances = append(instances, instance)

	config.OPA.CustomChecks = checks
	config.OPA.CustomCheckInstances = instances
	report, err = ProcessOPA(context.TODO(), object, "test", "", "Pod", "test", config)
	assert.NoError(t, err)
	assert.Equal(t, "opa", report.Report)
	err = json.Unmarshal(report.Contents, &reportObject)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(reportObject["ActionItems"].([]interface{})))

}

package opa

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/opa/pkg/kube"
	"github.com/fairwindsops/insights-plugins/plugins/opa/pkg/opa"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/models"
)

// TestProcessOPA runs all checks against the provided Custom Check
func TestProcessOPA(t *testing.T) {
	kube.SetFakeClient()
	config := models.Configuration{}
	iConfig := models.InsightsConfig{}
	checks := []opa.OPACustomCheck{}
	instances := []opa.CheckSetting{}
	config.OPA.CustomCheckInstances = instances
	config.OPA.CustomChecks = checks
	report, err := ProcessOPA(context.TODO(), nil, admission.Request{}, config, iConfig)
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
	req := admission.Request{
		AdmissionRequest: v1.AdmissionRequest{
			Name:      "test",
			Namespace: "test",
			RequestKind: &metav1.GroupVersionKind{
				Group: "",
				Kind:  "Pod",
			},
		},
	}
	check := opa.OPACustomCheck{
		Name:    "check2",
		Version: 2.0,
		Rego: `
package fairwinds
labelrequired[results] {
  provided := {label | input.metadata.labels[label]}
  required := {label | label := "hr"}
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
	config.OPA.CustomChecks = checks
	report, err = ProcessOPA(context.TODO(), object, req, config, iConfig)
	assert.NoError(t, err)
	assert.Equal(t, "opa", report.Report)
	err = json.Unmarshal(report.Contents, &reportObject)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(reportObject["ActionItems"].([]interface{})))
}

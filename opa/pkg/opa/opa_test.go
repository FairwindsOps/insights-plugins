package opa

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/fairwindsops/insights-plugins/opa/pkg/kube"
)

func TestOPAParse(t *testing.T) {
	kube.SetFakeClient()
	ctx := context.TODO()
	rego := `
        package fairwinds
        labelblock[description] {
          provided := {label | input.metadata.labels[label]}
          required := {label | label := input.parameters.labels[_]}
          missing := required - provided
          found := required - missing
          count(found) > 0
          description := sprintf("label %v is present", [found])
        }
	`

	params := map[string]interface{}{}
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]string{
				"foo": "bar",
			},
		},
	}

	results, err := runRegoForItem(ctx, rego, params, obj)
	assert.NoError(t, err)

	resource := unstructured.Unstructured{Object: obj}

	details := outputFormat{}

	ais, err := processResults(resource, results, "my-test", details)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(ais))

	params["labels"] = []string{"foo"}
	results, err = runRegoForItem(ctx, rego, params, obj)
	ais, err = processResults(resource, results, "my-test", details)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ais))
	assert.Equal(t, defaultSeverity, ais[0].Severity)
	assert.Equal(t, defaultTitle, ais[0].Title)
	assert.Equal(t, defaultRemediation, ais[0].Remediation)
	assert.Equal(t, defaultCategory, ais[0].Category)
	assert.Equal(t, "label {\"foo\"} is present", ais[0].Description)
}

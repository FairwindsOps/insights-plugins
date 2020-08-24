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
        annotationblock[description] {
          provided := {annotation | input.metadata.annotations[annotation]}
          required := {annotation | annotation := input.parameters.annotations[_]}
          missing := required - provided
          found := required - missing
          count(found) > 0
          description := sprintf("annotation %v is present", [found])
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
	assert.Equal(t, 0, len(ais))
}

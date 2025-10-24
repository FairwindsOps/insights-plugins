package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractValidatingPoliciesFromMessage(t *testing.T) {
	errorMessage := "admission webhook \"vpol.validate.kyverno.svc-fail\" denied the request: Policy check-labels failed: label 'environment' is required"
	result := ExtractValidatingPoliciesFromMessage(errorMessage)
	assert.Equal(t, 1, len(result))
	assert.Equal(t, map[string]map[string]string{
		"check-labels": {
			"check-labels": "admission webhook \"vpol.validate.kyverno.svc-fail\" denied the request: Policy check-labels failed: label 'environment' is required",
		},
	}, result)
}

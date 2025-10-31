package utils

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

func TestExtractAuditOnlyAllowedValidatingAdmissionPoliciesFromMessage(t *testing.T) {
	annotations := map[string]string{
		"validation.policy.admission.k8s.io/validation_failure": "[{\"message\":\"failed expression: object.spec.replicas >= 5\",\"policy\":\"check-deployment-replicas\",\"binding\":\"check-deployment-replicas-binding\",\"expressionIndex\":0,\"validationActions\":[\"Audit\"]}]",
	}
	result := ExtractAuditOnlyAllowedValidatingAdmissionPoliciesFromMessage(annotations)
	assert.Equal(t, 1, len(result))
	assert.Equal(t, map[string]map[string]string{
		"check-deployment-replicas": {
			"check-deployment-replicas": "[{\"message\":\"failed expression: object.spec.replicas >= 5\",\"policy\":\"check-deployment-replicas\",\"binding\":\"check-deployment-replicas-binding\",\"expressionIndex\":0,\"validationActions\":[\"Audit\"]}]",
		},
	}, result)
}

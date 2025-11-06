package utils

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
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

func TestCreateBlockedWatchedEventFromAuditEventFromFileName(t *testing.T) {
	jsonFile, err := os.Open("../samples/pods_blocked.json")
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer jsonFile.Close()

	var auditEvent models.AuditEvent
	json.NewDecoder(jsonFile).Decode(&auditEvent)
	watchedEvent := CreateBlockedWatchedEventFromAuditEvent(auditEvent)
	assert.NotNil(t, watchedEvent)
	assert.Equal(t, "Pod", watchedEvent.Kind)
	assert.Equal(t, "insights-agent", watchedEvent.Namespace)
	assert.Equal(t, "kyverno-policy-violation-Pod-workloads-29372898-vt4pm-cef08638-bd92-4d0f-b261-87863d98d271", watchedEvent.Name)
	assert.Equal(t, "cef08638-bd92-4d0f-b261-87863d98d271", watchedEvent.UID)
	assert.Equal(t, false, watchedEvent.Success)
	assert.Equal(t, true, watchedEvent.Blocked)
	assert.Equal(t, map[string]map[string]string{
		"james-require-labels": {
			"check-required-labels-james-2": "validation error: Required labels (app, version, environment) must be present. rule check-required-labels-james-2 failed at path /metadata/labels/app/",
		},
		"james-require-resource-limits": {
			"check-resource-limits-james-2": "validation error: All containers must have resource limits defined. rule check-resource-limits-james-2 failed at path /spec/containers/0/resources/limits/",
		},
	}, watchedEvent.Metadata["policies"])
}

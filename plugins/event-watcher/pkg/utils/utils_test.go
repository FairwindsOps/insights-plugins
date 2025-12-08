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

func TestExtractNamespacedValidatingPoliciesFromMessage(t *testing.T) {
	errorMessage := "admission webhook \"nvpol.validate.kyverno.svc-fail\" denied the request: Policy check-deployment-replicas failed: deployments must have at least 5 replicas for high availability"
	result := ExtractNamespacedValidatingPoliciesFromMessage(errorMessage)
	assert.Equal(t, 1, len(result))
	assert.Equal(t, map[string]map[string]string{
		"check-deployment-replicas": {
			"check-deployment-replicas": "admission webhook \"nvpol.validate.kyverno.svc-fail\" denied the request: Policy check-deployment-replicas failed: deployments must have at least 5 replicas for high availability",
		},
	}, result)
}

func TestExtractImageValidatingPoliciesFromMessage(t *testing.T) {
	errorMessage := "admission webhook \"ivpol.validate.kyverno.svc-fail\" denied the request: Policy require-signed-images error: failed to evaluate policy: Get \"https://untrusted.registry.io/v2/\": dial tcp: lookup untrusted.registry.io on 10.96.0.10:53: no such host"
	result := ExtractImageValidatingPoliciesFromMessage(errorMessage)
	assert.Equal(t, 1, len(result))
	assert.Equal(t, map[string]map[string]string{
		"require-signed-images": {
			"require-signed-images": "admission webhook \"ivpol.validate.kyverno.svc-fail\" denied the request: Policy require-signed-images error: failed to evaluate policy: Get \"https://untrusted.registry.io/v2/\": dial tcp: lookup untrusted.registry.io on 10.96.0.10:53: no such host",
		},
	}, result)
}

func TestExtractPoliciesDoNotMix(t *testing.T) {
	// Test that vpol message doesn't match nvpol or ivpol
	vpolMessage := "admission webhook \"vpol.validate.kyverno.svc-fail\" denied the request: Policy check-labels failed: label 'environment' is required"
	nvpolResult := ExtractNamespacedValidatingPoliciesFromMessage(vpolMessage)
	ivpolResult := ExtractImageValidatingPoliciesFromMessage(vpolMessage)
	assert.Equal(t, map[string]map[string]string{"unknown": {"unknown": vpolMessage}}, nvpolResult)
	assert.Equal(t, map[string]map[string]string{"unknown": {"unknown": vpolMessage}}, ivpolResult)

	// Test that nvpol message doesn't match vpol or ivpol
	nvpolMessage := "admission webhook \"nvpol.validate.kyverno.svc-fail\" denied the request: Policy check-deployment-replicas failed: deployments must have at least 5 replicas"
	vpolResult := ExtractValidatingPoliciesFromMessage(nvpolMessage)
	ivpolResult2 := ExtractImageValidatingPoliciesFromMessage(nvpolMessage)
	assert.Equal(t, map[string]map[string]string{"unknown": {"unknown": nvpolMessage}}, vpolResult)
	assert.Equal(t, map[string]map[string]string{"unknown": {"unknown": nvpolMessage}}, ivpolResult2)

	// Test that ivpol message doesn't match vpol or nvpol
	ivpolMessage := "admission webhook \"ivpol.validate.kyverno.svc-fail\" denied the request: Policy require-signed-images error: failed to evaluate policy"
	vpolResult2 := ExtractValidatingPoliciesFromMessage(ivpolMessage)
	nvpolResult2 := ExtractNamespacedValidatingPoliciesFromMessage(ivpolMessage)
	assert.Equal(t, map[string]map[string]string{"unknown": {"unknown": ivpolMessage}}, vpolResult2)
	assert.Equal(t, map[string]map[string]string{"unknown": {"unknown": ivpolMessage}}, nvpolResult2)
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

func TestIsValidatingPolicyViolation(t *testing.T) {
	// Test vpol detection
	vpolMessage := "admission webhook \"vpol.validate.kyverno.svc-fail\" denied the request: Policy check-labels failed: label 'environment' is required"
	assert.True(t, IsValidatingPolicyViolation(403, vpolMessage))
	assert.True(t, IsValidatingPolicyViolation(400, vpolMessage))

	// Test that vpol doesn't match when ivpol is present
	ivpolMessage := "admission webhook \"ivpol.validate.kyverno.svc-fail\" denied the request: Policy require-signed-images error: failed to evaluate policy"
	assert.False(t, IsValidatingPolicyViolation(403, ivpolMessage))

	// Test that vpol doesn't match when nvpol is present
	nvpolMessage := "admission webhook \"nvpol.validate.kyverno.svc-fail\" denied the request: Policy check-deployment-replicas failed: deployments must have at least 5 replicas"
	assert.False(t, IsValidatingPolicyViolation(403, nvpolMessage))

	// Test that low response codes don't match
	assert.False(t, IsValidatingPolicyViolation(200, vpolMessage))
	assert.False(t, IsValidatingPolicyViolation(399, vpolMessage))
}

func TestIsNamespacedValidatingPolicyViolation(t *testing.T) {
	// Test nvpol detection
	nvpolMessage := "admission webhook \"nvpol.validate.kyverno.svc-fail\" denied the request: Policy check-deployment-replicas failed: deployments must have at least 5 replicas for high availability"
	assert.True(t, IsNamespacedValidatingPolicyViolation(403, nvpolMessage))
	assert.True(t, IsNamespacedValidatingPolicyViolation(400, nvpolMessage))

	// Test that nvpol doesn't match vpol
	vpolMessage := "admission webhook \"vpol.validate.kyverno.svc-fail\" denied the request: Policy check-labels failed: label 'environment' is required"
	assert.False(t, IsNamespacedValidatingPolicyViolation(403, vpolMessage))

	// Test that nvpol doesn't match ivpol
	ivpolMessage := "admission webhook \"ivpol.validate.kyverno.svc-fail\" denied the request: Policy require-signed-images error: failed to evaluate policy"
	assert.False(t, IsNamespacedValidatingPolicyViolation(403, ivpolMessage))

	// Test that low response codes don't match
	assert.False(t, IsNamespacedValidatingPolicyViolation(200, nvpolMessage))
	assert.False(t, IsNamespacedValidatingPolicyViolation(399, nvpolMessage))
}

func TestIsImageValidatingPolicyViolation(t *testing.T) {
	// Test ivpol detection
	ivpolMessage := "admission webhook \"ivpol.validate.kyverno.svc-fail\" denied the request: Policy require-signed-images error: failed to evaluate policy: Get \"https://untrusted.registry.io/v2/\": dial tcp: lookup untrusted.registry.io on 10.96.0.10:53: no such host"
	assert.True(t, IsImageValidatingPolicyViolation(403, ivpolMessage))
	assert.True(t, IsImageValidatingPolicyViolation(400, ivpolMessage))

	// Test that ivpol doesn't match vpol
	vpolMessage := "admission webhook \"vpol.validate.kyverno.svc-fail\" denied the request: Policy check-labels failed: label 'environment' is required"
	assert.False(t, IsImageValidatingPolicyViolation(403, vpolMessage))

	// Test that ivpol doesn't match nvpol
	nvpolMessage := "admission webhook \"nvpol.validate.kyverno.svc-fail\" denied the request: Policy check-deployment-replicas failed: deployments must have at least 5 replicas"
	assert.False(t, IsImageValidatingPolicyViolation(403, nvpolMessage))

	// Test that low response codes don't match
	assert.False(t, IsImageValidatingPolicyViolation(200, ivpolMessage))
	assert.False(t, IsImageValidatingPolicyViolation(399, ivpolMessage))
}

func TestPolicyViolationDetectionDoNotMix(t *testing.T) {
	// Test that each policy type only matches its own detection function
	vpolMessage := "admission webhook \"vpol.validate.kyverno.svc-fail\" denied the request: Policy check-labels failed: label 'environment' is required"
	nvpolMessage := "admission webhook \"nvpol.validate.kyverno.svc-fail\" denied the request: Policy check-deployment-replicas failed: deployments must have at least 5 replicas"
	ivpolMessage := "admission webhook \"ivpol.validate.kyverno.svc-fail\" denied the request: Policy require-signed-images error: failed to evaluate policy"

	// vpol should only match IsValidatingPolicyViolation
	assert.True(t, IsValidatingPolicyViolation(403, vpolMessage))
	assert.False(t, IsNamespacedValidatingPolicyViolation(403, vpolMessage))
	assert.False(t, IsImageValidatingPolicyViolation(403, vpolMessage))

	// nvpol should only match IsNamespacedValidatingPolicyViolation
	assert.False(t, IsValidatingPolicyViolation(403, nvpolMessage))
	assert.True(t, IsNamespacedValidatingPolicyViolation(403, nvpolMessage))
	assert.False(t, IsImageValidatingPolicyViolation(403, nvpolMessage))

	// ivpol should only match IsImageValidatingPolicyViolation
	assert.False(t, IsValidatingPolicyViolation(403, ivpolMessage))
	assert.False(t, IsNamespacedValidatingPolicyViolation(403, ivpolMessage))
	assert.True(t, IsImageValidatingPolicyViolation(403, ivpolMessage))
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

func TestCreateBlockedWatchedEventFromAuditEventFromFileNamePlutoBlock(t *testing.T) {
	jsonFile, err := os.Open("../samples/pluto_block.json")
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
	assert.Equal(t, "kyverno-policy-violation-Pod-pluto-29373967-cmtdc-a8779987-0a41-482c-a28d-9fa27e8cb364", watchedEvent.Name)
	assert.Equal(t, "a8779987-0a41-482c-a28d-9fa27e8cb364", watchedEvent.UID)
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
	assert.Equal(t, "Blocked", watchedEvent.Metadata["action"])
	assert.Equal(t, "admission webhook \"validate.kyverno.svc-fail\" denied the request: \n\nresource Pod/insights-agent/pluto-29373967-cmtdc was blocked due to the following policies \n\njames-require-labels:\n  check-required-labels-james-2: 'validation error: Required labels (app, version,\n    environment) must be present. rule check-required-labels-james-2 failed at path\n    /metadata/labels/app/'\njames-require-resource-limits:\n  check-resource-limits-james-2: 'validation error: All containers must have resource\n    limits defined. rule check-resource-limits-james-2 failed at path /spec/containers/0/resources/limits/'\n", watchedEvent.Metadata["message"])
}

func TestCreateBlockedPolicyViolationEventFromPlutoBlock(t *testing.T) {
	jsonFile, err := os.Open("../samples/pluto_block.json")
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer jsonFile.Close()

	var auditEvent models.AuditEvent
	json.NewDecoder(jsonFile).Decode(&auditEvent)
	policyViolationEvent := CreateBlockedPolicyViolationEvent(auditEvent)
	assert.NotNil(t, policyViolationEvent)
	assert.Equal(t, "Pod", policyViolationEvent.ResourceType)
	assert.Equal(t, "insights-agent", policyViolationEvent.Namespace)
	assert.Equal(t, "kyverno-policy-violation-Pod-pluto-29373967-cmtdc-a8779987-0a41-482c-a28d-9fa27e8cb364", policyViolationEvent.Name)
	assert.Equal(t, "a8779987-0a41-482c-a28d-9fa27e8cb364", policyViolationEvent.AuditID)
	assert.Equal(t, map[string]map[string]string{
		"james-require-labels": {
			"check-required-labels-james-2": "validation error: Required labels (app, version, environment) must be present. rule check-required-labels-james-2 failed at path /metadata/labels/app/",
		},
		"james-require-resource-limits": {
			"check-resource-limits-james-2": "validation error: All containers must have resource limits defined. rule check-resource-limits-james-2 failed at path /spec/containers/0/resources/limits/",
		},
	}, policyViolationEvent.Policies)
}

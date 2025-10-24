package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
)

func TestPolicyViolationHandlerHandleBlockedViolation(t *testing.T) {
	// Set up test logger (slog is used by default)

	// Create test server to capture API calls
	var apiCalls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls = append(apiCalls, r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	// Create handler with test configuration
	config := models.InsightsConfig{
		Hostname:     server.URL,
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}

	handler := NewPolicyViolationHandler(config, 30, 60)

	// Create a blocked PolicyViolation event
	event := &event.WatchedEvent{
		EventVersion: 1,
		Timestamp:    time.Now().Unix(),
		EventType:    event.EventTypeAdded,
		ResourceType: "events",
		Namespace:    "default",
		Name:         "policy-violation-test",
		UID:          "test-uid-123",
		Data: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Event",
			"reason":     "PolicyViolation",
			"message":    "Pod default/nginx: [require-team-label] fail (blocked); validation error: The label 'team' is required for all Pods.",
			"involvedObject": map[string]interface{}{
				"kind":      "ValidatingAdmissionPolicy", // This makes it a ValidatingAdmissionPolicy event
				"name":      "require-team-label",
				"namespace": "",
			},
		},
		Metadata: map[string]interface{}{
			"name":      "policy-violation-test",
			"namespace": "default",
			"uid":       "test-uid-123",
		},
	}

	// Execute the handler
	err := handler.Handle(event)

	// Verify results
	assert.NoError(t, err)
	assert.Len(t, apiCalls, 1)
	assert.Equal(t, "/v0/organizations/test-org/clusters/test-cluster/data/watcher/policy-violations", apiCalls[0])
}

func TestPolicyViolationHandlerHandleNonBlockedViolation(t *testing.T) {
	// Set up test logger (slog is used by default)

	// Create test server to capture API calls
	var apiCalls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls = append(apiCalls, r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	// Create handler with test configuration
	config := models.InsightsConfig{
		Hostname:     server.URL,
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}

	handler := NewPolicyViolationHandler(config, 30, 60)

	// Create a non-blocked PolicyViolation event
	event := &event.WatchedEvent{
		EventVersion: 1,
		Timestamp:    time.Now().Unix(),
		EventType:    event.EventTypeAdded,
		ResourceType: "events",
		Namespace:    "default",
		Name:         "policy-violation-warning",
		UID:          "test-uid-456",
		Data: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Event",
			"reason":     "PolicyViolation",
			"message":    "Pod default/nginx: [require-team-label] warn validation warning: The label 'team' is recommended for all Pods.",
			"involvedObject": map[string]interface{}{
				"kind":      "Pod",
				"name":      "nginx",
				"namespace": "default",
			},
		},
		Metadata: map[string]interface{}{
			"name":      "policy-violation-warning",
			"namespace": "default",
			"uid":       "test-uid-456",
		},
	}

	// Execute the handler
	err := handler.Handle(event)

	// Verify results - should not call API for non-blocked violations (only blocked policy violations are sent)
	assert.NoError(t, err)
	assert.Len(t, apiCalls, 1)
	assert.Equal(t, "/v0/organizations/test-org/clusters/test-cluster/data/watcher/policy-violations", apiCalls[0])
}

func TestPolicyViolationHandlerHandleBlockedKyvernoPolicyEvent(t *testing.T) {
	// Set up test logger (slog is used by default)

	// Create test server to capture API calls
	var apiCalls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls = append(apiCalls, r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	// Create handler with test configuration
	config := models.InsightsConfig{
		Hostname:     server.URL,
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}

	handler := NewPolicyViolationHandler(config, 30, 60)

	// Create a blocked regular Kyverno policy event
	event := &event.WatchedEvent{
		EventVersion: 1,
		Timestamp:    time.Now().Unix(),
		EventType:    event.EventTypeAdded,
		ResourceType: "events",
		Namespace:    "default",
		Name:         "kyverno-policy-violation-test",
		UID:          "test-uid-999",
		Data: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Event",
			"reason":     "PolicyViolation",
			"message":    "policy disallow-host-path/disallow-host-path fail (blocked): HostPath volumes are forbidden.",
			"involvedObject": map[string]interface{}{
				"kind":      "Deployment", // This makes it a regular Kyverno policy event
				"name":      "nginx",
				"namespace": "default",
			},
		},
		Metadata: map[string]interface{}{
			"name":      "kyverno-policy-violation-test",
			"namespace": "default",
			"uid":       "test-uid-999",
		},
	}

	// Execute the handler
	err := handler.Handle(event)

	// Verify results - should call API for blocked policy violations (any type)
	assert.NoError(t, err)
	assert.Len(t, apiCalls, 1)
	assert.Equal(t, "/v0/organizations/test-org/clusters/test-cluster/data/watcher/policy-violations", apiCalls[0])
}

func TestPolicyViolationHandlerParsePolicyMessage(t *testing.T) {
	tests := []struct {
		name                 string
		message              string
		expectedPolicies     map[string]map[string]string
		expectedPolicyResult string
		expectedBlocked      bool
		expectedError        bool
	}{
		{
			name:    "blocked policy violation",
			message: "Error from server: error when creating \"deploy.yaml\": admission webhook \"validate.kyverno.svc-fail\" denied the request: \n\nresource Deployment/default/nginx-deployment was blocked due to the following policies \n\njames-disallow-privileged-containers:\n  check-privileged-james-1: 'validation error: Privileged containers are not allowed.\n    rule check-privileged-james-1 failed at path /spec/containers/'james-require-labels:\n  check-required-labels-james-1: 'validation error: Required labels (app, version,\n    environment) must be present. rule check-required-labels-james-1 failed at path\n    /metadata/labels/environment/'james-require-resource-limits:\n  check-resource-limits-james-1: 'validation error: All containers must have resource\n    limits defined. rule check-resource-limits-james-1 failed at path /spec/containers/'",
			expectedPolicies: map[string]map[string]string{
				"james-disallow-privileged-containers": {
					"message": "fail, Privileged containers are not allowed.",
				},
				"james-require-labels": {
					"message": "fail, Required labels (app, version, environment) must be present.",
				},
				"james-require-resource-limits": {
					"message": "fail, All containers must have resource limits defined.",
				},
			},
			expectedPolicyResult: "fail",
			expectedBlocked:      true,
			expectedError:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policies := ExtractPoliciesFromMessage(tt.message)

			if tt.expectedError {
				assert.Equal(t, tt.expectedPolicies, policies)
			}
		})
	}
}

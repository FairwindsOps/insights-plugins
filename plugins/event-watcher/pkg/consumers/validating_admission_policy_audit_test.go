package consumers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestAuditOnlyAllowedValidatingAdmissionPolicyHandlerHandle(t *testing.T) {
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
	handler := NewAuditOnlyAllowedValidatingAdmissionPolicyHandler(config, 30, 60)
	// Use a unique UID to avoid bigcache deduplication
	uniqueUID := "test-uid-audit-vap-" + time.Now().Format("20060102150405")
	err := handler.Handle(&models.WatchedEvent{
		EventVersion: 1,
		Timestamp:    time.Now().Unix(),
		EventType:    models.EventTypeAdded,
		Kind:         "Deployment",
		Namespace:    "default",
		Name:         "audit-only-vap-test",
		UID:          uniqueUID,
		Success:      true,
		Blocked:      false,
		Data: map[string]any{
			"annotations": map[string]string{
				"validation.policy.admission.k8s.io/validation_failure": "[{\"message\":\"failed expression: object.spec.replicas >= 5\",\"policy\":\"check-deployment-replicas\",\"binding\":\"check-deployment-replicas-binding\",\"expressionIndex\":0,\"validationActions\":[\"Audit\"]}]",
			},
		},
		Metadata: map[string]any{
			"policyResult": "audit",
		},
	})
	assert.NoError(t, err)
	assert.Len(t, apiCalls, 1)
	assert.Equal(t, "/v0/organizations/test-org/clusters/test-cluster/data/watcher/policy-violations", apiCalls[0])
}

func TestAuditOnlyAllowedValidatingAdmissionPolicyHandlerParsePolicyMessage(t *testing.T) {
	tests := []struct {
		name             string
		annotations      map[string]string
		expectedPolicies map[string]map[string]string
		expectedError    bool
	}{
		{
			name: "audit only validating admission policy",
			annotations: map[string]string{
				"validation.policy.admission.k8s.io/validation_failure": "[{\"message\":\"failed expression: object.spec.replicas >= 5\",\"policy\":\"check-deployment-replicas\",\"binding\":\"check-deployment-replicas-binding\",\"expressionIndex\":0,\"validationActions\":[\"Audit\"]}]",
			},
			expectedPolicies: map[string]map[string]string{
				"check-deployment-replicas": {
					"check-deployment-replicas": "[{\"message\":\"failed expression: object.spec.replicas >= 5\",\"policy\":\"check-deployment-replicas\",\"binding\":\"check-deployment-replicas-binding\",\"expressionIndex\":0,\"validationActions\":[\"Audit\"]}]",
				},
			},
			expectedError: false,
		},
		{
			name: "another audit only policy",
			annotations: map[string]string{
				"validation.policy.admission.k8s.io/validation_failure": "[{\"message\":\"failed expression: object.metadata.labels.env == 'prod'\",\"policy\":\"require-prod-label\",\"binding\":\"require-prod-label-binding\",\"expressionIndex\":0,\"validationActions\":[\"Audit\"]}]",
			},
			expectedPolicies: map[string]map[string]string{
				"require-prod-label": {
					"require-prod-label": "[{\"message\":\"failed expression: object.metadata.labels.env == 'prod'\",\"policy\":\"require-prod-label\",\"binding\":\"require-prod-label-binding\",\"expressionIndex\":0,\"validationActions\":[\"Audit\"]}]",
				},
			},
			expectedError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policies := utils.ExtractAuditOnlyAllowedValidatingAdmissionPoliciesFromMessage(tt.annotations)
			assert.Equal(t, tt.expectedPolicies, policies)
		})
	}
}

func TestAuditOnlyAllowedValidatingAdmissionPolicyHandlerExtractError(t *testing.T) {
	config := models.InsightsConfig{
		Hostname:     "http://localhost",
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}
	handler := NewAuditOnlyAllowedValidatingAdmissionPolicyHandler(config, 30, 60)

	// Test nil event
	err := handler.Handle(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "watchedEvent is nil")

	// Test event with nil data
	err = handler.Handle(&models.WatchedEvent{
		Data: nil,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no annotations field in event or annotations is not a map")

	// Test event with missing annotations
	err = handler.Handle(&models.WatchedEvent{
		Data: map[string]any{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no annotations field in event or annotations is not a map")
}


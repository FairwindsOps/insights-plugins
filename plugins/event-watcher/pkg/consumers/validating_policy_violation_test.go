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

func TestValidatingPolicyViolationHandlerHandle(t *testing.T) {
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
	handler := NewValidatingPolicyViolationHandler(config, 30, 60)
	// Use a unique UID to avoid bigcache deduplication
	uniqueUID := "test-uid-vpol-" + time.Now().Format("20060102150405")
	err := handler.Handle(&models.WatchedEvent{
		EventVersion: 1,
		Timestamp:    time.Now().Unix(),
		EventType:    models.EventTypeAdded,
		Kind:         "Pod",
		Namespace:    "default",
		Name:         "vpol-violation-test",
		UID:          uniqueUID,
		Success:      false,
		Blocked:      true,
		Data: map[string]any{
			"message": "admission webhook \"vpol.validate.kyverno.svc-fail\" denied the request: Policy check-labels failed: label 'environment' is required",
		},
		Metadata: map[string]any{
			"policyResult": "fail",
		},
	})
	assert.NoError(t, err)
	assert.Len(t, apiCalls, 1)
	assert.Equal(t, "/v0/organizations/test-org/clusters/test-cluster/data/watcher/policy-violations", apiCalls[0])
}

func TestValidatingPolicyViolationHandlerParsePolicyMessage(t *testing.T) {
	tests := []struct {
		name             string
		message          string
		expectedPolicies map[string]map[string]string
		expectedError    bool
	}{
		{
			name:    "blocked validating policy violation",
			message: "admission webhook \"vpol.validate.kyverno.svc-fail\" denied the request: Policy check-labels failed: label 'environment' is required",
			expectedPolicies: map[string]map[string]string{
				"check-labels": {
					"check-labels": "admission webhook \"vpol.validate.kyverno.svc-fail\" denied the request: Policy check-labels failed: label 'environment' is required",
				},
			},
			expectedError: false,
		},
		{
			name:    "another validating policy",
			message: "admission webhook \"vpol.validate.kyverno.svc-fail\" denied the request: Policy require-resource-limits failed: all containers must have resource limits",
			expectedPolicies: map[string]map[string]string{
				"require-resource-limits": {
					"require-resource-limits": "admission webhook \"vpol.validate.kyverno.svc-fail\" denied the request: Policy require-resource-limits failed: all containers must have resource limits",
				},
			},
			expectedError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policies := utils.ExtractValidatingPoliciesFromMessage(tt.message)
			assert.Equal(t, tt.expectedPolicies, policies)
		})
	}
}

func TestValidatingPolicyViolationHandlerExtractError(t *testing.T) {
	config := models.InsightsConfig{
		Hostname:     "http://localhost",
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}
	handler := NewValidatingPolicyViolationHandler(config, 30, 60)

	// Test nil event
	err := handler.Handle(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "watchedEvent is nil")

	// Test event with nil data
	err = handler.Handle(&models.WatchedEvent{
		Data: nil,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "event data is nil")

	// Test event with empty message
	err = handler.Handle(&models.WatchedEvent{
		Data: map[string]any{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no message field in event or message is empty")
}


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

func TestValidatingAdmissionPolicyHandlerHandle(t *testing.T) {

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
	handler := NewValidatingAdmissionPolicyViolationHandler(config, 30, 60)
	err := handler.Handle(&models.WatchedEvent{
		EventVersion: 1,
		Timestamp:    time.Now().Unix(),
		EventType:    models.EventTypeAdded,
		Kind:         "events",
		Namespace:    "default",
		Name:         "validating-admission-policy-violation-test",
		UID:          "test-uid-123",
		Data: map[string]any{
			"message": "deployments.apps \"nginx-deployment\" is forbidden: ValidatingAdmissionPolicy 'check-deployment-replicas' with binding 'check-deployment-replicas-binding' denied request: failed expression: object.spec.replicas >= 5",
		},
		Metadata: map[string]any{
			"policyResult": "fail",
		},
	})
	assert.NoError(t, err)
	assert.Len(t, apiCalls, 1)
	assert.Equal(t, "/v0/organizations/test-org/clusters/test-cluster/data/watcher/policy-violations", apiCalls[0])
}

func TestValidatingAdmissionPolicyHandlerParsePolicyMessage(t *testing.T) {
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
			message: "deployments.apps \"nginx-deployment\" is forbidden: ValidatingAdmissionPolicy 'check-deployment-replicas' with binding 'check-deployment-replicas-binding' denied request: failed expression: object.spec.replicas >= 5",
			expectedPolicies: map[string]map[string]string{
				"check-deployment-replicas": {
					"check-deployment-replicas": "deployments.apps \"nginx-deployment\" is forbidden: ValidatingAdmissionPolicy 'check-deployment-replicas' with binding 'check-deployment-replicas-binding' denied request: failed expression: object.spec.replicas >= 5",
				},
			},
			expectedPolicyResult: "",
			expectedBlocked:      true,
			expectedError:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policies := utils.ExtractValidatingAdmissionPoliciesFromMessage(tt.message)
			assert.Equal(t, tt.expectedPolicies, policies)
		})
	}
}

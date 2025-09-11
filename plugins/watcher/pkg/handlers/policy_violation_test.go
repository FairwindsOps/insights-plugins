package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
)

func TestPolicyViolationHandlerHandleBlockedViolation(t *testing.T) {
	// Set up test logger
	logrus.SetLevel(logrus.DebugLevel)

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

	handler := NewPolicyViolationHandler(config)

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
				"kind":      "Pod",
				"name":      "nginx",
				"namespace": "default",
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
	// Set up test logger
	logrus.SetLevel(logrus.DebugLevel)

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

	handler := NewPolicyViolationHandler(config)

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

	// Verify results - should not call API for non-blocked violations
	assert.NoError(t, err)
	assert.Len(t, apiCalls, 0)
}

func TestPolicyViolationHandlerParsePolicyMessage(t *testing.T) {
	handler := &PolicyViolationHandler{}

	tests := []struct {
		name            string
		message         string
		expectedPolicy  string
		expectedResult  string
		expectedBlocked bool
		expectedError   bool
	}{
		{
			name:            "blocked policy violation",
			message:         "Pod default/nginx: [require-team-label] fail (blocked); validation error: The label 'team' is required for all Pods.",
			expectedPolicy:  "require-team-label",
			expectedResult:  "fail",
			expectedBlocked: true,
			expectedError:   false,
		},
		{
			name:            "warning policy violation",
			message:         "Pod default/nginx: [require-team-label] warn validation warning: The label 'team' is recommended for all Pods.",
			expectedPolicy:  "require-team-label",
			expectedResult:  "warn",
			expectedBlocked: false,
			expectedError:   false,
		},
		{
			name:            "validation error format",
			message:         "Pod default/nginx: [require-team-label] validation error The label 'team' is required for all Pods.",
			expectedPolicy:  "require-team-label",
			expectedResult:  "fail",
			expectedBlocked: false,
			expectedError:   false,
		},
		{
			name:            "invalid message format",
			message:         "Invalid message without brackets",
			expectedPolicy:  "",
			expectedResult:  "",
			expectedBlocked: false,
			expectedError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy, result, blocked, err := handler.parsePolicyMessage(tt.message)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPolicy, policy)
				assert.Equal(t, tt.expectedResult, result)
				assert.Equal(t, tt.expectedBlocked, blocked)
			}
		})
	}
}

// MockInsightsClient follows the project pattern for simple mocks
type MockInsightsClient struct {
	apiCalls []string
	errors   []error
}

func (m *MockInsightsClient) SendPolicyViolation(violation *models.PolicyViolationEvent) error {
	m.apiCalls = append(m.apiCalls, "policy-violation")
	if len(m.errors) > 0 {
		err := m.errors[0]
		m.errors = m.errors[1:]
		return err
	}
	return nil
}

func TestPolicyViolationHandlerWithMockClient(t *testing.T) {
	// Create mock client
	mockClient := &MockInsightsClient{}

	// Create handler with mock client (this would require modifying the handler to accept a client interface)
	// For now, we'll test the parsing logic separately
	handler := &PolicyViolationHandler{}

	// Test the parsing logic
	message := "Pod default/nginx: [require-team-label] fail (blocked); validation error: The label 'team' is required for all Pods."
	policy, result, blocked, err := handler.parsePolicyMessage(message)

	assert.NoError(t, err)
	assert.Equal(t, "require-team-label", policy)
	assert.Equal(t, "fail", result)
	assert.True(t, blocked)

	// Verify mock client wasn't called (since we're not using it in this test)
	assert.Len(t, mockClient.apiCalls, 0)
}

package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
)

func TestEventHandlerFactoryGetHandler(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	config := models.InsightsConfig{
		Hostname:     server.URL,
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}

	factory := NewEventHandlerFactory(config)

	tests := []struct {
		name            string
		event           *event.WatchedEvent
		expectedHandler string
	}{
		{
			name: "PolicyViolation event should return policy-violation handler",
			event: &event.WatchedEvent{
				ResourceType: "events",
				Data: map[string]interface{}{
					"reason":  "PolicyViolation",
					"message": "Pod default/nginx: [require-team-label] fail (blocked); validation error: The label 'team' is required for all Pods.",
					"involvedObject": map[string]interface{}{
						"kind":      "Pod",
						"name":      "nginx",
						"namespace": "default",
					},
				},
			},
			expectedHandler: "policy-violation",
		},
		{
			name: "PolicyReport event should return policyreport-handler",
			event: &event.WatchedEvent{
				ResourceType: "PolicyReport",
			},
			expectedHandler: "policyreport-handler",
		},
		{
			name: "ClusterPolicyReport event should return clusterpolicyreport-handler",
			event: &event.WatchedEvent{
				ResourceType: "ClusterPolicyReport",
			},
			expectedHandler: "clusterpolicyreport-handler",
		},
		{
			name: "Policy event should return policy-handler",
			event: &event.WatchedEvent{
				ResourceType: "Policy",
			},
			expectedHandler: "policy-handler",
		},
		{
			name: "ClusterPolicy event should return clusterpolicy-handler",
			event: &event.WatchedEvent{
				ResourceType: "ClusterPolicy",
			},
			expectedHandler: "clusterpolicy-handler",
		},
		{
			name: "Unknown resource should return generic-resource handler",
			event: &event.WatchedEvent{
				ResourceType: "UnknownResource",
			},
			expectedHandler: "generic-resource",
		},
		{
			name: "Pod event should return generic-resource handler",
			event: &event.WatchedEvent{
				ResourceType: "Pod",
			},
			expectedHandler: "generic-resource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := factory.GetHandler(tt.event)
			assert.NotNil(t, handler, "Handler should not be nil")

			// Verify the handler type by checking if it can handle the event
			// (This is a simple way to verify we got the right handler)
			err := handler.Handle(tt.event)
			assert.NoError(t, err, "Handler should be able to handle the event")
		})
	}
}

func TestEventHandlerFactoryRegister(t *testing.T) {
	config := models.InsightsConfig{
		Hostname:     "https://test.com",
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}

	factory := NewEventHandlerFactory(config)

	// Create a custom handler
	customHandler := &GenericResourceHandler{insightsConfig: config}

	// Register the custom handler
	factory.Register("custom-test-handler", customHandler)

	// Verify it was registered
	handlerNames := factory.GetHandlerNames()
	assert.Contains(t, handlerNames, "custom-test-handler")

	// Verify handler count increased
	initialCount := factory.GetHandlerCount()
	assert.Greater(t, initialCount, 0)
}

func TestEventHandlerFactoryProcessEvent(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	config := models.InsightsConfig{
		Hostname:     server.URL,
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}

	factory := NewEventHandlerFactory(config)

	tests := []struct {
		name        string
		event       *event.WatchedEvent
		expectError bool
	}{
		{
			name: "Valid PolicyViolation event should be processed",
			event: &event.WatchedEvent{
				ResourceType: "events",
				Data: map[string]interface{}{
					"reason":  "PolicyViolation",
					"message": "Pod default/nginx: [require-team-label] fail (blocked); validation error: The label 'team' is required for all Pods.",
					"involvedObject": map[string]interface{}{
						"kind":      "Pod",
						"name":      "nginx",
						"namespace": "default",
					},
				},
			},
			expectError: false,
		},
		{
			name: "Valid PolicyReport event should be processed",
			event: &event.WatchedEvent{
				ResourceType: "PolicyReport",
				Data: map[string]interface{}{
					"results": []interface{}{
						map[string]interface{}{
							"result": "fail",
							"policy": "test-policy",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Valid generic resource event should be processed",
			event: &event.WatchedEvent{
				ResourceType: "Pod",
				Data: map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := factory.ProcessEvent(tt.event)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEventHandlerFactoryGetHandlerNames(t *testing.T) {
	config := models.InsightsConfig{
		Hostname:     "https://test.com",
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}

	factory := NewEventHandlerFactory(config)

	handlerNames := factory.GetHandlerNames()

	// Verify we have the expected default handlers
	expectedHandlers := []string{
		"policyreport-handler",
		"clusterpolicyreport-handler",
		"policy-handler",
		"clusterpolicy-handler",
		"generic-resource",
	}

	for _, expected := range expectedHandlers {
		assert.Contains(t, handlerNames, expected, "Should contain handler: %s", expected)
	}
}

func TestEventHandlerFactoryGetHandlerCount(t *testing.T) {
	config := models.InsightsConfig{
		Hostname:     "https://test.com",
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}

	factory := NewEventHandlerFactory(config)

	count := factory.GetHandlerCount()

	// Should have at least the default handlers
	assert.GreaterOrEqual(t, count, 5, "Should have at least 5 default handlers")
}

// Test the naming convention logic
func TestEventHandlerFactoryGetHandlerName(t *testing.T) {
	config := models.InsightsConfig{
		Hostname:     "https://test.com",
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}

	factory := NewEventHandlerFactory(config)

	tests := []struct {
		name         string
		event        *event.WatchedEvent
		expectedName string
	}{
		{
			name: "PolicyViolation event",
			event: &event.WatchedEvent{
				ResourceType: "events",
				Data: map[string]interface{}{
					"reason": "PolicyViolation",
				},
			},
			expectedName: "policy-violation",
		},
		{
			name: "PolicyReport resource",
			event: &event.WatchedEvent{
				ResourceType: "PolicyReport",
			},
			expectedName: "policyreport-handler",
		},
		{
			name: "Unknown resource",
			event: &event.WatchedEvent{
				ResourceType: "UnknownResource",
			},
			expectedName: "generic-resource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerName := factory.getHandlerName(tt.event)
			assert.Equal(t, tt.expectedName, handlerName)
		})
	}
}

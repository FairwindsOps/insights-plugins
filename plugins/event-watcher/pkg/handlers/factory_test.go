package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
)

func TestEventHandlerFactoryRegister(t *testing.T) {
	config := models.InsightsConfig{
		Hostname:     "https://test.com",
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}

	scheme := runtime.NewScheme()
	factory := NewEventHandlerFactory(config, fake.NewSimpleClientset(), dynamicfake.NewSimpleDynamicClient(scheme), 30, 60, false)

	// Create a custom handler
	customHandler := &PolicyViolationHandler{insightsConfig: config}

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

	scheme := runtime.NewScheme()
	factory := NewEventHandlerFactory(config, fake.NewSimpleClientset(), dynamicfake.NewSimpleDynamicClient(scheme), 30, 60, false)

	tests := []struct {
		name        string
		event       *event.WatchedEvent
		expectError bool
	}{
		{
			name: "Valid Kyverno PolicyViolation event should be processed",
			event: &event.WatchedEvent{
				ResourceType: "events",
				Data: map[string]interface{}{
					"reason":  "PolicyViolation",
					"message": "Error from server: error when creating \"deploy.yaml\": admission webhook \"validate.kyverno.svc-fail\" denied the request: \n\nresource Deployment/default/nginx-deployment was blocked due to the following policies \n\njames-disallow-privileged-containers:\n  check-privileged-james-1: 'validation error: Privileged containers are not allowed.\n    rule check-privileged-james-1 failed at path /spec/containers/'james-require-labels:\n  check-required-labels-james-1: 'validation error: Required labels (app, version,\n    environment) must be present. rule check-required-labels-james-1 failed at path\n    /metadata/labels/environment/'james-require-resource-limits:\n  check-resource-limits-james-1: 'validation error: All containers must have resource\n    limits defined. rule check-resource-limits-james-1 failed at path /spec/containers/'",
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
			name: "Valid Kyverno PolicyReport event should be processed",
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

	scheme := runtime.NewScheme()
	factory := NewEventHandlerFactory(config, fake.NewSimpleClientset(), dynamicfake.NewSimpleDynamicClient(scheme), 30, 60, false)

	handlerNames := factory.GetHandlerNames()

	// Verify we have the expected default handlers
	expectedHandlers := []string{
		"kyverno-policy-violation",
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

	scheme := runtime.NewScheme()
	factory := NewEventHandlerFactory(config, fake.NewSimpleClientset(), dynamicfake.NewSimpleDynamicClient(scheme), 30, 60, false)

	count := factory.GetHandlerCount()

	// Should have at least the default handlers
	assert.GreaterOrEqual(t, count, 1, "Should have at least 1 default handler")
}

// Test the naming convention logic
func TestEventHandlerFactoryGetHandlerName(t *testing.T) {
	config := models.InsightsConfig{
		Hostname:     "https://test.com",
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}

	scheme := runtime.NewScheme()
	factory := NewEventHandlerFactory(config, fake.NewSimpleClientset(), dynamicfake.NewSimpleDynamicClient(scheme), 30, 60, false)

	tests := []struct {
		name         string
		event        *event.WatchedEvent
		expectedName string
	}{
		{
			name: "Kyverno PolicyViolation event",
			event: &event.WatchedEvent{
				Name:         "kyverno-policy-violation-Deployment-nginx-deployment-5c0888f1-bdd0-4681-9aba-5b734c267df2",
				ResourceType: "Deployment",
				Namespace:    "default",
				UID:          "5c0888f1-bdd0-4681-9aba-5b734c267df2",
				EventTime:    "2025-10-23T10:23:37.369146Z",
				Timestamp:    time.Now().UTC().Unix(),
				Data: map[string]interface{}{
					"responseStatus": map[string]interface{}{
						"code":    400,
						"status":  "Failure",
						"message": "admission webhook \"validate.kyverno.svc-fail\" denied the request: \n\nresource Deployment/default/nginx-deployment was blocked due to the following policies \n\njames-disallow-privileged-containers:\n  check-privileged-james-1: 'validation error: Privileged containers are not allowed.\n    rule check-privileged-james-1 failed at path /spec/containers/'\n\njames-require-labels:\n  check-required-labels-james-1: 'validation error: Required labels (app, version,\n    environment) must be present. rule check-required-labels-james-1 failed at path\n    /metadata/labels/environment/'\njames-require-resource-limits:\n  check-resource-limits-james-1: 'validation error: All containers must have resource  \n    limits defined. rule check-resource-limits-james-1 failed at path /spec/containers/'",
					},
				},
				Metadata: map[string]interface{}{
					"audit_id":      "5c0888f1-bdd0-4681-9aba-5b734c267df2",
					"policies":      map[string]interface{}{},
					"resource_name": "nginx-deployment",
					"namespace":     "default",
					"action":        "Blocked",
					"message":       "admission webhook \"validate.kyverno.svc-fail\" denied the request: \n\nresource Deployment/default/nginx-deployment was blocked due to the following policies \n\njames-disallow-privileged-containers:\n  check-privileged-james-1: 'validation error: Privileged containers are not allowed.\n    rule check-privileged-james-1 failed at path /spec/containers/'\n\njames-require-labels:\n  check-required-labels-james-1: 'validation error: Required labels (app, version,\n    environment) must be present. rule check-required-labels-james-1 failed at path\n    /metadata/labels/environment/'\njames-require-resource-limits:\n  check-resource-limits-james-1: 'validation error: All containers must have resource  \n    limits defined. rule check-resource-limits-james-1 failed at path /spec/containers/'",
					"timestamp":     "2025-10-23T10:23:37.369146Z",
					"event_time":    "2025-10-23T10:23:37.369146Z",
				},
			},
			expectedName: "kyverno-policy-violation",
		},
		{
			name: "Kyverno ClusterPolicy resource",
			event: &event.WatchedEvent{
				ResourceType: "ClusterPolicy",
			},
			expectedName: "",
		},
		{
			name: "Kyverno PolicyReport resource",
			event: &event.WatchedEvent{
				ResourceType: "PolicyReport",
			},
			expectedName: "",
		},
		{
			name: "Kyverno Unknown resource should return no handler",
			event: &event.WatchedEvent{
				ResourceType: "UnknownResource",
			},
			expectedName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerName := factory.getHandlerName(tt.event)
			assert.Equal(t, tt.expectedName, handlerName)
		})
	}
}

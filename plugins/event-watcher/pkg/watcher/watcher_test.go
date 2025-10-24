package watcher

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/handlers"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
)

func TestWatcherHandlerFactory(t *testing.T) {
	// Set up test logger (slog is used by default)

	// Create test server to capture API calls
	var apiCalls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls = append(apiCalls, r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	// Create test configuration
	config := models.InsightsConfig{
		Hostname:     server.URL,
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}

	// Create handler factory directly (following project pattern)
	scheme := runtime.NewScheme()
	handlerFactory := handlers.NewEventHandlerFactory(config, fake.NewSimpleClientset(), dynamicfake.NewSimpleDynamicClient(scheme), 30, 60, false)
	assert.NotNil(t, handlerFactory)

	// Test ValidatingAdmissionPolicy event processing
	t.Run("ValidatingAdmissionPolicy event should trigger API call", func(t *testing.T) {
		// Create a ValidatingAdmissionPolicy event
		policyViolationEvent := &event.WatchedEvent{
			EventVersion: 1,
			Timestamp:    time.Now().Unix(),
			EventType:    event.EventTypeAdded,
			ResourceType: "events",
			Namespace:    "default",
			Name:         "kyverno-policy-violation-ValidatingAdmissionPolicy-require-team-label-test-uid-123",
			UID:          "test-uid-123",
			Data: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Event",
				"reason":     "PolicyViolation",
				"message":    "admission webhook \"validate.kyverno.svc-fail\" denied the request: \n\nresource ValidatingAdmissionPolicy/default/require-team-label was blocked due to the following policies \n\njames-disallow-privileged-containers:\n  check-privileged-james-1: 'validation error: Privileged containers are not allowed.\n    rule check-privileged-james-1 failed at path /spec/containers/'\n\njames-require-labels:\n  check-required-labels-james-1: 'validation error: Required labels (app, version,\n    environment) must be present. rule check-required-labels-james-1 failed at path\n    /metadata/labels/environment/'\njames-require-resource-limits:\n  check-resource-limits-james-1: 'validation error: All containers must have resource  \n    limits defined. rule check-resource-limits-james-1 failed at path /spec/containers/'",
				"involvedObject": map[string]interface{}{
					"kind":      "ValidatingAdmissionPolicy", // This makes it a ValidatingAdmissionPolicy event
					"name":      "require-team-label",
					"namespace": "",
				},
			},
			Metadata: map[string]interface{}{
				"name":      "kyverno-policy-violation-ValidatingAdmissionPolicy-require-team-label-test-uid-123",
				"namespace": "default",
				"uid":       "test-uid-123",
			},
		}

		// Process the event
		err := handlerFactory.ProcessEvent(policyViolationEvent)
		assert.NoError(t, err)

		// Verify API was called
		assert.Len(t, apiCalls, 1)
		assert.Equal(t, "/v0/organizations/test-org/clusters/test-cluster/data/watcher/policy-violations", apiCalls[0])
	})

	// Test PolicyReport event processing
	t.Run("PolicyReport event should be processed", func(t *testing.T) {
		// Reset API calls
		apiCalls = []string{}

		// Create a PolicyReport event
		policyReportEvent := &event.WatchedEvent{
			EventVersion: 1,
			Timestamp:    time.Now().Unix(),
			EventType:    event.EventTypeAdded,
			ResourceType: "PolicyReport",
			Namespace:    "default",
			Name:         "policy-report-test",
			UID:          "test-uid-789",
			Data: map[string]interface{}{
				"apiVersion": "wgpolicyk8s.io/v1alpha2",
				"kind":       "PolicyReport",
				"results": []interface{}{
					map[string]interface{}{
						"result":  "fail",
						"policy":  "require-team-label",
						"message": "Missing required label",
					},
					map[string]interface{}{
						"result":  "warn",
						"policy":  "recommend-labels",
						"message": "Missing recommended label",
					},
				},
			},
			Metadata: map[string]interface{}{
				"name":      "policy-report-test",
				"namespace": "default",
				"uid":       "test-uid-789",
			},
		}

		// Process the event
		err := handlerFactory.ProcessEvent(policyReportEvent)
		assert.NoError(t, err)

		// PolicyReport handler should not call API, just log
		assert.Len(t, apiCalls, 0)
	})
}

// Simple test for handler factory creation (following project patterns)
func TestEventHandlerFactory_Creation(t *testing.T) {
	config := models.InsightsConfig{
		Hostname:     "https://test.com",
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}

	scheme := runtime.NewScheme()
	factory := handlers.NewEventHandlerFactory(config, fake.NewSimpleClientset(), dynamicfake.NewSimpleDynamicClient(scheme), 30, 60, false)
	assert.NotNil(t, factory)
	assert.Greater(t, factory.GetHandlerCount(), 0)
}

// Test backpressure configuration
func TestBackpressureConfig(t *testing.T) {
	config := BackpressureConfig{
		MaxRetries:           5,
		RetryDelay:           50 * time.Millisecond,
		MetricsLogInterval:   10 * time.Second,
		EnableMetricsLogging: true,
	}

	assert.Equal(t, 5, config.MaxRetries)
	assert.Equal(t, 50*time.Millisecond, config.RetryDelay)
	assert.Equal(t, 10*time.Second, config.MetricsLogInterval)
	assert.True(t, config.EnableMetricsLogging)
}

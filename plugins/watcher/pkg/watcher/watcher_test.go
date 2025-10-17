package watcher

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/handlers"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
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
			Name:         "validatingadmissionpolicy-violation-test",
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
				"name":      "validatingadmissionpolicy-violation-test",
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

	// Test non-blocked PolicyViolation event
	t.Run("Non-blocked PolicyViolation event should not trigger API call", func(t *testing.T) {
		// Reset API calls
		apiCalls = []string{}

		// Create a non-blocked PolicyViolation event
		nonBlockedEvent := &event.WatchedEvent{
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

		// Process the event
		err := handlerFactory.ProcessEvent(nonBlockedEvent)
		assert.NoError(t, err)

		// Verify API was not called (only blocked violations are sent)
		assert.Len(t, apiCalls, 0)
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

// Test backpressure handling with small buffer
func TestWatcherBackpressureHandling(t *testing.T) {
	// Set up test logger (slog is used by default)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	// Create watcher with very small buffer to trigger backpressure
	backpressureConfig := BackpressureConfig{
		MaxRetries:           2,
		RetryDelay:           10 * time.Millisecond,
		MetricsLogInterval:   1 * time.Second,
		EnableMetricsLogging: false, // Disable for test
	}

	watcher, err := NewWatcherWithBackpressure(config, "local", "", nil, 1, 30, 60, false, backpressureConfig)
	assert.NoError(t, err)
	assert.NotNil(t, watcher)

	// Get initial metrics
	metrics := watcher.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, 1, metrics.ChannelCapacity)

	// Test that metrics are working
	metrics.RecordEventProcessed()
	processed, dropped := metrics.GetTotalEvents()
	assert.Equal(t, int64(1), processed)
	assert.Equal(t, int64(0), dropped)
}

// Test metrics functionality
func TestWatcherMetrics(t *testing.T) {
	// Create test configuration
	config := models.InsightsConfig{
		Hostname:     "https://test.com",
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}

	// Create watcher
	watcher, err := NewWatcher(config, "local", "", nil, 100, 30, 60, false)
	assert.NoError(t, err)
	assert.NotNil(t, watcher)

	// Get metrics
	metrics := watcher.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, 100, metrics.ChannelCapacity)

	// Test metrics recording
	metrics.RecordEventInChannel()
	metrics.RecordEventProcessed()
	metrics.RecordEventDropped()

	processed, dropped := metrics.GetTotalEvents()
	assert.Equal(t, int64(1), processed)
	assert.Equal(t, int64(1), dropped)
	assert.Equal(t, int64(1), metrics.EventsInChannel)

	// Test channel utilization
	utilization := metrics.GetChannelUtilization()
	assert.Equal(t, 1.0, utilization) // 1/100 * 100 = 1%

	// Test uptime
	uptime := metrics.GetUptime()
	assert.True(t, uptime > 0)
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

func TestNewWatcher(t *testing.T) {
	tests := []struct {
		name               string
		insightsConfig     models.InsightsConfig
		logSource          string
		auditLogPath       string
		cloudwatchConfig   *models.CloudWatchConfig
		eventBufferSize    int
		httpTimeoutSeconds int
		rateLimitPerMinute int
		consoleMode        bool
		expectError        bool
	}{
		{
			name: "valid local mode configuration",
			insightsConfig: models.InsightsConfig{
				Hostname:     "https://test.com",
				Organization: "test-org",
				Cluster:      "test-cluster",
				Token:        "test-token",
			},
			logSource:          "local",
			auditLogPath:       "/var/log/audit.log",
			eventBufferSize:    100,
			httpTimeoutSeconds: 30,
			rateLimitPerMinute: 60,
			consoleMode:        false,
			expectError:        false,
		},
		{
			name: "valid cloudwatch mode configuration",
			insightsConfig: models.InsightsConfig{
				Hostname:     "https://test.com",
				Organization: "test-org",
				Cluster:      "test-cluster",
				Token:        "test-token",
			},
			logSource: "cloudwatch",
			cloudwatchConfig: &models.CloudWatchConfig{
				LogGroupName:  "/aws/eks/test/cluster",
				Region:        "us-east-1",
				FilterPattern: "{ $.stage = \"ResponseComplete\" }",
				BatchSize:     100,
				PollInterval:  "30s",
				MaxMemoryMB:   512,
			},
			eventBufferSize:    100,
			httpTimeoutSeconds: 30,
			rateLimitPerMinute: 60,
			consoleMode:        false,
			expectError:        false,
		},
		{
			name: "console mode configuration",
			insightsConfig: models.InsightsConfig{
				Hostname:     "",
				Organization: "",
				Cluster:      "",
				Token:        "",
			},
			logSource:          "local",
			eventBufferSize:    100,
			httpTimeoutSeconds: 30,
			rateLimitPerMinute: 60,
			consoleMode:        true,
			expectError:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			watcher, err := NewWatcher(tt.insightsConfig, tt.logSource, tt.auditLogPath, tt.cloudwatchConfig, tt.eventBufferSize, tt.httpTimeoutSeconds, tt.rateLimitPerMinute, tt.consoleMode)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, watcher)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, watcher)
				assert.NotNil(t, watcher.GetMetrics())
			}
		})
	}
}

func TestNewWatcherWithBackpressure(t *testing.T) {
	config := models.InsightsConfig{
		Hostname:     "https://test.com",
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}

	backpressureConfig := BackpressureConfig{
		MaxRetries:           3,
		RetryDelay:           100 * time.Millisecond,
		MetricsLogInterval:   30 * time.Second,
		EnableMetricsLogging: true,
	}

	watcher, err := NewWatcherWithBackpressure(config, "local", "", nil, 100, 30, 60, false, backpressureConfig)
	assert.NoError(t, err)
	assert.NotNil(t, watcher)
	assert.NotNil(t, watcher.GetMetrics())
}

func TestWatcherGetResourcesToWatch(t *testing.T) {
	config := models.InsightsConfig{
		Hostname:     "https://test.com",
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}

	watcher, err := NewWatcher(config, "local", "", nil, 100, 30, 60, false)
	require.NoError(t, err)

	assert.NotNil(t, watcher)
}

func TestWatcherStop(t *testing.T) {
	config := models.InsightsConfig{
		Hostname:     "https://test.com",
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}

	watcher, err := NewWatcher(config, "local", "", nil, 100, 30, 60, false)
	require.NoError(t, err)

	// Stop should not panic
	assert.NotPanics(t, func() {
		watcher.Stop()
	})
}

func TestWatcherMetricsIntegration(t *testing.T) {
	config := models.InsightsConfig{
		Hostname:     "https://test.com",
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}

	watcher, err := NewWatcher(config, "local", "", nil, 100, 30, 60, false)
	require.NoError(t, err)

	metrics := watcher.GetMetrics()
	require.NotNil(t, metrics)

	// Test that metrics are properly initialized
	assert.Equal(t, 100, metrics.ChannelCapacity)
	assert.Equal(t, int64(0), metrics.EventsProcessed)
	assert.Equal(t, int64(0), metrics.EventsDropped)
}

func TestWatcherDifferentBufferSizes(t *testing.T) {
	config := models.InsightsConfig{
		Hostname:     "https://test.com",
		Organization: "test-org",
		Cluster:      "test-cluster",
		Token:        "test-token",
	}

	bufferSizes := []int{1, 10, 100, 1000}

	for _, bufferSize := range bufferSizes {
		t.Run(fmt.Sprintf("buffer_size_%d", bufferSize), func(t *testing.T) {
			watcher, err := NewWatcher(config, "local", "", nil, bufferSize, 30, 60, false)
			assert.NoError(t, err)
			assert.NotNil(t, watcher)

			metrics := watcher.GetMetrics()
			assert.Equal(t, bufferSize, metrics.ChannelCapacity)
		})
	}
}

package health

import (
	"context"
	"testing"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/metrics"
	"github.com/stretchr/testify/assert"
)

// mockWatcher is a mock implementation of the MetricsProvider for testing
type mockWatcher struct {
	metrics *metrics.Metrics
}

func (m *mockWatcher) GetMetrics() *metrics.Metrics {
	return m.metrics
}

func TestNewWatcherChecker(t *testing.T) {
	mockWatcher := &mockWatcher{
		metrics: metrics.NewMetrics(1000),
	}

	checker := NewWatcherCheckerWithMinUptime(mockWatcher, 0)
	assert.NotNil(t, checker)
	assert.Equal(t, "kubernetes-watcher", checker.GetName())
}

func TestWatcherChecker_CheckHealth(t *testing.T) {
	tests := []struct {
		name         string
		setupMetrics func(*metrics.Metrics)
		expectError  bool
		errorMsg     string
	}{
		{
			name: "healthy watcher with low drop rate",
			setupMetrics: func(m *metrics.Metrics) {
				// Simulate some processed events with low drop rate
				for i := 0; i < 100; i++ {
					m.RecordEventProcessed()
				}
				for i := 0; i < 5; i++ {
					m.RecordEventDropped()
				}
			},
			expectError: false,
		},
		{
			name: "healthy watcher with no events",
			setupMetrics: func(m *metrics.Metrics) {
				// No events processed yet
			},
			expectError: false,
		},
		{
			name: "unhealthy watcher with high drop rate",
			setupMetrics: func(m *metrics.Metrics) {
				// Simulate high drop rate (>50%)
				for i := 0; i < 10; i++ {
					m.RecordEventProcessed()
				}
				for i := 0; i < 60; i++ {
					m.RecordEventDropped()
				}
			},
			expectError: true,
			errorMsg:    "high event drop rate",
		},
		{
			name: "unhealthy watcher with high channel utilization",
			setupMetrics: func(m *metrics.Metrics) {
				// Simulate high channel utilization (>90%)
				for i := 0; i < 950; i++ {
					m.RecordEventInChannel()
				}
			},
			expectError: true,
			errorMsg:    "high channel utilization",
		},
		{
			name: "unhealthy watcher still starting up",
			setupMetrics: func(m *metrics.Metrics) {
				// Metrics just created, uptime < 5 seconds
				// This should be handled by the uptime check
			},
			expectError: false, // With zero min uptime, this should pass
			errorMsg:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWatcher := &mockWatcher{
				metrics: metrics.NewMetrics(1000),
			}

			// Setup metrics based on test case
			tt.setupMetrics(mockWatcher.metrics)

			checker := NewWatcherCheckerWithMinUptime(mockWatcher, 0)

			ctx := context.Background()
			err := checker.CheckHealth(ctx)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWatcherChecker_CheckHealth_ContextTimeout(t *testing.T) {
	mockWatcher := &mockWatcher{
		metrics: metrics.NewMetrics(1000),
	}

	checker := NewWatcherCheckerWithMinUptime(mockWatcher, 0)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := checker.CheckHealth(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestWatcherChecker_GetName(t *testing.T) {
	mockWatcher := &mockWatcher{
		metrics: metrics.NewMetrics(1000),
	}

	checker := NewWatcherCheckerWithMinUptime(mockWatcher, 0)
	assert.Equal(t, "kubernetes-watcher", checker.GetName())
}

func TestWatcherChecker_CheckHealth_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		setupMetrics func(*metrics.Metrics)
		expectError  bool
	}{
		{
			name: "exactly 50% drop rate should be healthy",
			setupMetrics: func(m *metrics.Metrics) {
				for i := 0; i < 50; i++ {
					m.RecordEventProcessed()
				}
				for i := 0; i < 50; i++ {
					m.RecordEventDropped()
				}
			},
			expectError: false,
		},
		{
			name: "exactly 90% channel utilization should be healthy",
			setupMetrics: func(m *metrics.Metrics) {
				for i := 0; i < 900; i++ {
					m.RecordEventInChannel()
				}
			},
			expectError: false,
		},
		{
			name: "zero capacity channel should be healthy",
			setupMetrics: func(m *metrics.Metrics) {
				// Create metrics with zero capacity
				mockWatcher := &mockWatcher{
					metrics: metrics.NewMetrics(0),
				}
				checker := NewWatcherCheckerWithMinUptime(mockWatcher, 0)
				err := checker.CheckHealth(context.Background())
				assert.NoError(t, err)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWatcher := &mockWatcher{
				metrics: metrics.NewMetrics(1000),
			}

			tt.setupMetrics(mockWatcher.metrics)

			checker := NewWatcherCheckerWithMinUptime(mockWatcher, 0)

			err := checker.CheckHealth(context.Background())

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWatcherChecker_CheckHealth_WithTimeDelay(t *testing.T) {
	mockWatcher := &mockWatcher{
		metrics: metrics.NewMetrics(1000),
	}

	checker := NewWatcherChecker(mockWatcher) // Use default 5 second minimum uptime

	// Initially should fail due to short uptime
	err := checker.CheckHealth(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "still starting up")

	// Wait for uptime to be > 5 seconds
	time.Sleep(6 * time.Second)

	// Now should pass
	err = checker.CheckHealth(context.Background())
	assert.NoError(t, err)
}

func TestWatcherChecker_CheckHealth_UptimeCheck(t *testing.T) {
	mockWatcher := &mockWatcher{
		metrics: metrics.NewMetrics(1000),
	}

	// Test with default minimum uptime (5 seconds)
	checker := NewWatcherChecker(mockWatcher)

	// Should fail due to short uptime
	err := checker.CheckHealth(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "still starting up")

	// Test with zero minimum uptime
	checkerZero := NewWatcherCheckerWithMinUptime(mockWatcher, 0)

	// Should pass with zero minimum uptime
	err = checkerZero.CheckHealth(context.Background())
	assert.NoError(t, err)
}

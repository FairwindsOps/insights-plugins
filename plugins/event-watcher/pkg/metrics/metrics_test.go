package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewMetrics(t *testing.T) {
	capacity := 100
	metrics := NewMetrics(capacity)

	assert.NotNil(t, metrics)
	assert.Equal(t, capacity, metrics.ChannelCapacity)
	assert.Equal(t, int64(0), metrics.EventsProcessed)
	assert.Equal(t, int64(0), metrics.EventsDropped)
	assert.Equal(t, int64(0), metrics.EventsInChannel)
}

func TestRecordEventProcessed(t *testing.T) {
	metrics := NewMetrics(100)

	metrics.RecordEventProcessed()
	metrics.RecordEventProcessed()

	processed, dropped := metrics.GetTotalEvents()
	assert.Equal(t, int64(2), processed)
	assert.Equal(t, int64(0), dropped)
}

func TestRecordEventDropped(t *testing.T) {
	metrics := NewMetrics(100)

	metrics.RecordEventDropped()
	metrics.RecordEventDropped()
	metrics.RecordEventDropped()

	processed, dropped := metrics.GetTotalEvents()
	assert.Equal(t, int64(0), processed)
	assert.Equal(t, int64(3), dropped)
}

func TestRecordEventInChannel(t *testing.T) {
	metrics := NewMetrics(100)

	metrics.RecordEventInChannel()
	metrics.RecordEventInChannel()
	metrics.RecordEventInChannel()

	assert.Equal(t, int64(3), metrics.EventsInChannel)
}

func TestRecordEventOutChannel(t *testing.T) {
	metrics := NewMetrics(100)

	// Add some events
	metrics.RecordEventInChannel()
	metrics.RecordEventInChannel()
	metrics.RecordEventInChannel()

	// Remove some events
	metrics.RecordEventOutChannel()
	metrics.RecordEventOutChannel()

	assert.Equal(t, int64(1), metrics.EventsInChannel)
}

func TestRecordEventOutChannelDoesNotGoNegative(t *testing.T) {
	metrics := NewMetrics(100)

	// Try to remove events when none exist
	metrics.RecordEventOutChannel()
	metrics.RecordEventOutChannel()

	assert.Equal(t, int64(0), metrics.EventsInChannel)
}

func TestGetChannelUtilization(t *testing.T) {
	metrics := NewMetrics(100)

	// 0% utilization
	assert.Equal(t, 0.0, metrics.GetChannelUtilization())

	// 50% utilization
	metrics.EventsInChannel = 50
	assert.Equal(t, 50.0, metrics.GetChannelUtilization())

	// 100% utilization
	metrics.EventsInChannel = 100
	assert.Equal(t, 100.0, metrics.GetChannelUtilization())
}

func TestGetChannelUtilizationZeroCapacity(t *testing.T) {
	metrics := NewMetrics(0)

	// Should return 0 for zero capacity
	assert.Equal(t, 0.0, metrics.GetChannelUtilization())
}

func TestRecordProcessingDuration(t *testing.T) {
	metrics := NewMetrics(100)

	duration := 100 * time.Millisecond
	metrics.RecordProcessingDuration(duration)

	assert.Equal(t, duration, metrics.ProcessingDuration)
}

func TestGetUptime(t *testing.T) {
	metrics := NewMetrics(100)

	// Should be very small initially
	uptime := metrics.GetUptime()
	assert.True(t, uptime < time.Second)

	// Wait a bit and check again
	time.Sleep(10 * time.Millisecond)
	uptime = metrics.GetUptime()
	assert.True(t, uptime >= 10*time.Millisecond)
}

func TestReset(t *testing.T) {
	metrics := NewMetrics(100)

	// Add some data
	metrics.RecordEventProcessed()
	metrics.RecordEventDropped()
	metrics.RecordEventInChannel()
	metrics.RecordProcessingDuration(100 * time.Millisecond)

	// Reset
	metrics.Reset()

	// Check that everything is reset
	processed, dropped := metrics.GetTotalEvents()
	assert.Equal(t, int64(0), processed)
	assert.Equal(t, int64(0), dropped)
	assert.Equal(t, int64(0), metrics.EventsInChannel)
	assert.Equal(t, time.Duration(0), metrics.ProcessingDuration)
}

func TestConcurrentAccess(t *testing.T) {
	metrics := NewMetrics(100)

	// Test concurrent access
	done := make(chan bool, 10)

	// Start multiple goroutines
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				metrics.RecordEventProcessed()
				metrics.RecordEventInChannel()
				metrics.RecordEventOutChannel()
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check that metrics are consistent
	processed, dropped := metrics.GetTotalEvents()
	assert.Equal(t, int64(1000), processed) // 10 goroutines * 100 events
	assert.Equal(t, int64(0), dropped)
	assert.Equal(t, int64(0), metrics.EventsInChannel) // All events should be processed
}

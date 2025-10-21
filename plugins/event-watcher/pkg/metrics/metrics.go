package metrics

import (
	"log/slog"
	"sync"
	"time"
)

// Metrics tracks various performance metrics for the watcher
type Metrics struct {
	// Event processing metrics
	EventsProcessed    int64
	EventsDropped      int64
	EventsInChannel    int64
	ProcessingDuration time.Duration

	// Channel metrics
	ChannelCapacity    int
	ChannelUtilization float64

	// Rate metrics
	EventsPerSecond   float64
	ProcessingRate    float64
	DroppedEventsRate float64

	// Timestamps for rate calculations
	lastEventTime     time.Time
	lastProcessedTime time.Time
	lastDroppedTime   time.Time
	startTime         time.Time

	// Mutex for thread safety
	mu sync.RWMutex
}

// NewMetrics creates a new metrics instance
func NewMetrics(channelCapacity int) *Metrics {
	now := time.Now()
	return &Metrics{
		ChannelCapacity:   channelCapacity,
		startTime:         now,
		lastEventTime:     now,
		lastProcessedTime: now,
		lastDroppedTime:   now,
	}
}

// RecordEventProcessed records a successfully processed event
func (m *Metrics) RecordEventProcessed() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.EventsProcessed++
	m.lastProcessedTime = time.Now()
	m.updateRates()
}

// RecordEventDropped records a dropped event
func (m *Metrics) RecordEventDropped() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.EventsDropped++
	m.lastDroppedTime = time.Now()
	m.updateRates()
}

// RecordEventInChannel records an event being added to the channel
func (m *Metrics) RecordEventInChannel() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.EventsInChannel++
	m.lastEventTime = time.Now()
}

// RecordEventOutChannel records an event being removed from the channel
func (m *Metrics) RecordEventOutChannel() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.EventsInChannel > 0 {
		m.EventsInChannel--
	}
}

// RecordProcessingDuration records the time taken to process an event
func (m *Metrics) RecordProcessingDuration(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ProcessingDuration = duration
}

// GetChannelUtilization returns the current channel utilization percentage
func (m *Metrics) GetChannelUtilization() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.ChannelCapacity == 0 {
		return 0
	}
	return float64(m.EventsInChannel) / float64(m.ChannelCapacity) * 100
}

// GetEventsPerSecond returns the current events per second rate
func (m *Metrics) GetEventsPerSecond() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.EventsPerSecond
}

// GetProcessingRate returns the current processing rate (events/second)
func (m *Metrics) GetProcessingRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.ProcessingRate
}

// GetDroppedEventsRate returns the current dropped events rate (events/second)
func (m *Metrics) GetDroppedEventsRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.DroppedEventsRate
}

// GetTotalEvents returns the total number of events processed and dropped
func (m *Metrics) GetTotalEvents() (processed, dropped int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.EventsProcessed, m.EventsDropped
}

// GetUptime returns the uptime since metrics were created
func (m *Metrics) GetUptime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return time.Since(m.startTime)
}

// LogMetrics logs current metrics to the logger
func (m *Metrics) LogMetrics() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	slog.Info("Watcher metrics",
		"events_processed", m.EventsProcessed,
		"events_dropped", m.EventsDropped,
		"events_in_channel", m.EventsInChannel,
		"channel_capacity", m.ChannelCapacity,
		"channel_utilization", m.GetChannelUtilization(),
		"events_per_second", m.EventsPerSecond,
		"processing_rate", m.ProcessingRate,
		"dropped_events_rate", m.DroppedEventsRate,
		"uptime", m.GetUptime())
}

// updateRates calculates current rates based on recent activity
func (m *Metrics) updateRates() {
	now := time.Now()

	// Calculate events per second based on total events and uptime
	uptime := now.Sub(m.startTime)
	if uptime > 0 {
		totalEvents := m.EventsProcessed + m.EventsDropped
		m.EventsPerSecond = float64(totalEvents) / uptime.Seconds()
	}

	// Calculate processing rate based on recent processed events
	if !m.lastProcessedTime.IsZero() {
		timeSinceLastProcessed := now.Sub(m.lastProcessedTime)
		if timeSinceLastProcessed > 0 {
			// This is a simplified rate calculation
			// In a real implementation, you might want to use a sliding window
			m.ProcessingRate = 1.0 / timeSinceLastProcessed.Seconds()
		}
	}

	// Calculate dropped events rate based on recent dropped events
	if !m.lastDroppedTime.IsZero() {
		timeSinceLastDropped := now.Sub(m.lastDroppedTime)
		if timeSinceLastDropped > 0 {
			// This is a simplified rate calculation
			m.DroppedEventsRate = 1.0 / timeSinceLastDropped.Seconds()
		}
	}
}

// Reset resets all metrics to zero
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.EventsProcessed = 0
	m.EventsDropped = 0
	m.EventsInChannel = 0
	m.ProcessingDuration = 0
	m.EventsPerSecond = 0
	m.ProcessingRate = 0
	m.DroppedEventsRate = 0
	m.startTime = time.Now()
	m.lastEventTime = time.Now()
	m.lastProcessedTime = time.Now()
	m.lastDroppedTime = time.Now()
}

// GetMetrics returns the metrics instance itself (for health checker compatibility)
func (m *Metrics) GetMetrics() *Metrics {
	return m
}

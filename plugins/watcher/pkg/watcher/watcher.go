package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/client"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/handlers"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/metrics"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
)

// BackpressureConfig defines backpressure handling configuration
type BackpressureConfig struct {
	// MaxRetries is the maximum number of retries when channel is full
	MaxRetries int
	// RetryDelay is the delay between retries
	RetryDelay time.Duration
	// MetricsLogInterval is the interval for logging metrics
	MetricsLogInterval time.Duration
	// EnableMetricsLogging enables periodic metrics logging
	EnableMetricsLogging bool
}

// Watcher manages watching multiple event sources generically
type Watcher struct {
	// Core components
	eventSourceManager *EventSourceManager
	handlerFactory     *handlers.EventHandlerFactory
	metrics            *metrics.Metrics

	// Event processing
	eventChannel chan *event.WatchedEvent
	stopCh       chan struct{}
	wg           sync.WaitGroup

	// Configuration
	insightsConfig     models.InsightsConfig
	backpressureConfig BackpressureConfig
}

// NewWatcher creates a new generic watcher
func NewWatcher(insightsConfig models.InsightsConfig, logSource, auditLogPath string, cloudwatchConfig *models.CloudWatchConfig, eventBufferSize, httpTimeoutSeconds, rateLimitPerMinute int, consoleMode bool) (*Watcher, error) {
	return NewWatcherWithBackpressure(insightsConfig, logSource, auditLogPath, cloudwatchConfig, eventBufferSize, httpTimeoutSeconds, rateLimitPerMinute, consoleMode, BackpressureConfig{
		MaxRetries:           3,
		RetryDelay:           100 * time.Millisecond,
		MetricsLogInterval:   30 * time.Second,
		EnableMetricsLogging: true,
	})
}

// NewWatcherWithBackpressure creates a new generic watcher with custom backpressure configuration
func NewWatcherWithBackpressure(insightsConfig models.InsightsConfig, logSource, auditLogPath string, cloudwatchConfig *models.CloudWatchConfig, eventBufferSize, httpTimeoutSeconds, rateLimitPerMinute int, consoleMode bool, backpressureConfig BackpressureConfig) (*Watcher, error) {
	// Create Kubernetes client
	kubeClient, err := client.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Create handler factory
	handlerFactory := handlers.NewEventHandlerFactory(insightsConfig, kubeClient.KubeInterface, kubeClient.DynamicInterface, httpTimeoutSeconds, rateLimitPerMinute, consoleMode)

	// Create event channel
	eventChannel := make(chan *event.WatchedEvent, eventBufferSize)

	// Create metrics instance
	metricsInstance := metrics.NewMetrics(eventBufferSize)

	// Create event source manager
	eventSourceManager := NewEventSourceManager()

	// Create event source factory
	factory := NewEventSourceFactory()

	// Build event source configurations
	configs := BuildEventSourceConfigs(insightsConfig, kubeClient, logSource, auditLogPath, cloudwatchConfig, eventChannel)

	// Create event sources using factory
	sources, err := factory.CreateEventSources(configs)
	if err != nil {
		return nil, fmt.Errorf("failed to create event sources: %w", err)
	}

	// Add event sources to manager
	for i, source := range sources {
		config := configs[i]
		eventSourceManager.AddEventSource(string(config.Type), source)
	}

	w := &Watcher{
		eventSourceManager: eventSourceManager,
		handlerFactory:     handlerFactory,
		metrics:            metricsInstance,
		eventChannel:       eventChannel,
		stopCh:             make(chan struct{}),
		insightsConfig:     insightsConfig,
		backpressureConfig: backpressureConfig,
	}

	return w, nil
}

// Start begins watching all event sources
func (w *Watcher) Start(ctx context.Context) error {
	slog.Info("Starting generic watcher")

	// Start all event sources
	if err := w.eventSourceManager.StartAll(ctx); err != nil {
		return fmt.Errorf("failed to start event sources: %w", err)
	}

	// Start event processor
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.processEvents()
	}()

	// Start metrics logging if enabled
	if w.backpressureConfig.EnableMetricsLogging {
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			w.logMetricsPeriodically()
		}()
	}

	slog.Info("Generic watcher started successfully",
		"event_sources", w.eventSourceManager.GetEventSourceCount(),
		"source_names", w.eventSourceManager.GetEventSourceNames())

	return nil
}

// Stop stops the watcher
func (w *Watcher) Stop() {
	slog.Info("Stopping generic watcher")
	close(w.stopCh)

	// Stop all event sources
	w.eventSourceManager.StopAll()

	// Wait for all goroutines to finish
	w.wg.Wait()

	close(w.eventChannel)
	slog.Info("Generic watcher stopped")
}

// processEvents processes events from all sources
func (w *Watcher) processEvents() {
	for {
		select {
		case <-w.stopCh:
			return
		case watchedEvent, ok := <-w.eventChannel:
			if !ok {
				return
			}

			// Record event being removed from channel
			w.metrics.RecordEventOutChannel()

			// Record processing start time
			startTime := time.Now()

			watchedEvent.LogEvent()

			if err := w.handlerFactory.ProcessEvent(watchedEvent); err != nil {
				slog.Error("Failed to process event through handlers - this may indicate issues with event handler logic or API communication",
					"error", err,
					"event_type", watchedEvent.EventType,
					"resource_type", watchedEvent.ResourceType,
					"namespace", watchedEvent.Namespace,
					"name", watchedEvent.Name,
					"error_type", fmt.Sprintf("%T", err))
			}

			// Record processing completion and duration
			processingDuration := time.Since(startTime)
			w.metrics.RecordProcessingDuration(processingDuration)
			w.metrics.RecordEventProcessed()
		}
	}
}

// logMetricsPeriodically logs metrics at regular intervals
func (w *Watcher) logMetricsPeriodically() {
	ticker := time.NewTicker(w.backpressureConfig.MetricsLogInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.metrics.LogMetrics()
		}
	}
}

// GetMetrics returns the current metrics
func (w *Watcher) GetMetrics() *metrics.Metrics {
	return w.metrics
}

// GetEventSourceCount returns the number of active event sources
func (w *Watcher) GetEventSourceCount() int {
	return w.eventSourceManager.GetEventSourceCount()
}

// GetEventSourceNames returns the names of all event sources
func (w *Watcher) GetEventSourceNames() []string {
	return w.eventSourceManager.GetEventSourceNames()
}

// GetSupportedEventSourceTypes returns all supported event source types
func (w *Watcher) GetSupportedEventSourceTypes() []EventSourceType {
	factory := NewEventSourceFactory()
	return factory.GetSupportedTypes()
}

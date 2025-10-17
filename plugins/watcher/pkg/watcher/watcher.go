package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/client"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/handlers"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/metrics"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
)

// Watcher manages watching Kubernetes resources
type Watcher struct {
	client             *client.Client
	watchers           map[string]watch.Interface
	informers          map[string]cache.SharedInformer
	eventChannel       chan *event.WatchedEvent
	stopCh             chan struct{}
	mu                 sync.RWMutex
	wg                 sync.WaitGroup
	handlerFactory     *handlers.EventHandlerFactory
	insightsConfig     models.InsightsConfig
	auditLogHandler    *handlers.AuditLogHandler
	auditLogPath       string
	logSource          string
	cloudwatchConfig   *models.CloudWatchConfig
	cloudwatchHandler  *handlers.CloudWatchHandler
	metrics            *metrics.Metrics
	backpressureConfig BackpressureConfig
}

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

// NewWatcher creates a new Kubernetes watcher
func NewWatcher(insightsConfig models.InsightsConfig, logSource, auditLogPath string, cloudwatchConfig *models.CloudWatchConfig, eventBufferSize, httpTimeoutSeconds, rateLimitPerMinute int, consoleMode bool) (*Watcher, error) {
	return NewWatcherWithBackpressure(insightsConfig, logSource, auditLogPath, cloudwatchConfig, eventBufferSize, httpTimeoutSeconds, rateLimitPerMinute, consoleMode, BackpressureConfig{
		MaxRetries:           3,
		RetryDelay:           100 * time.Millisecond,
		MetricsLogInterval:   30 * time.Second,
		EnableMetricsLogging: true,
	})
}

// NewWatcherWithBackpressure creates a new Kubernetes watcher with custom backpressure configuration
func NewWatcherWithBackpressure(insightsConfig models.InsightsConfig, logSource, auditLogPath string, cloudwatchConfig *models.CloudWatchConfig, eventBufferSize, httpTimeoutSeconds, rateLimitPerMinute int, consoleMode bool, backpressureConfig BackpressureConfig) (*Watcher, error) {
	kubeClient, err := client.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Create handler factory with HTTP timeout and rate limiting
	handlerFactory := handlers.NewEventHandlerFactory(insightsConfig, kubeClient.KubeInterface, kubeClient.DynamicInterface, httpTimeoutSeconds, rateLimitPerMinute, consoleMode)

	eventChannel := make(chan *event.WatchedEvent, eventBufferSize)

	// Create metrics instance
	metricsInstance := metrics.NewMetrics(eventBufferSize)

	// Create audit log handler if audit log path is provided
	var auditLogHandler *handlers.AuditLogHandler
	if auditLogPath != "" {
		auditLogHandler = handlers.NewAuditLogHandler(insightsConfig, kubeClient.KubeInterface, auditLogPath, eventChannel)
	}

	// Create CloudWatch handler if CloudWatch is enabled
	var cloudwatchHandler *handlers.CloudWatchHandler
	if logSource == "cloudwatch" && cloudwatchConfig != nil {
		var err error
		cloudwatchHandler, err = handlers.NewCloudWatchHandler(insightsConfig, *cloudwatchConfig, eventChannel)
		if err != nil {
			return nil, fmt.Errorf("failed to create CloudWatch handler: %w", err)
		}
	}

	w := &Watcher{
		client:             kubeClient,
		watchers:           make(map[string]watch.Interface),
		informers:          make(map[string]cache.SharedInformer),
		eventChannel:       eventChannel,
		stopCh:             make(chan struct{}),
		handlerFactory:     handlerFactory,
		insightsConfig:     insightsConfig,
		auditLogHandler:    auditLogHandler,
		auditLogPath:       auditLogPath,
		logSource:          logSource,
		cloudwatchConfig:   cloudwatchConfig,
		cloudwatchHandler:  cloudwatchHandler,
		metrics:            metricsInstance,
		backpressureConfig: backpressureConfig,
	}

	return w, nil
}

// Start begins watching Kubernetes resources
func (w *Watcher) Start(ctx context.Context) error {
	slog.Info("Starting Kubernetes watcher")

	// Define resources to watch
	resources := w.getResourcesToWatch()

	// Start watching each resource type
	for _, resourceType := range resources {
		if err := w.watchResource(ctx, resourceType); err != nil {
			slog.Warn("Failed to start watching resource - this may be due to missing RBAC permissions, resource not available, or API server issues",
				"error", err,
				"resource", resourceType,
				"error_type", fmt.Sprintf("%T", err))
			continue
		}
		slog.Info("Started watching resource", "resource", resourceType)
	}

	// Check existing ValidatingAdmissionPolicies for audit duplicates
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		if err := w.checkExistingPolicies(); err != nil {
			slog.Error("Failed to check existing policies for audit duplicates", "error", err)
		}
	}()

	// Start audit log monitoring if enabled
	if w.auditLogHandler != nil {
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			if err := w.auditLogHandler.Start(ctx); err != nil {
				slog.Error("Failed to start audit log monitoring", "error", err)
			}
		}()
	}

	// Start CloudWatch log monitoring if enabled
	if w.cloudwatchHandler != nil {
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			if err := w.cloudwatchHandler.Start(ctx); err != nil {
				slog.Error("Failed to start CloudWatch log monitoring", "error", err)
			}
		}()
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

	// Start informers
	for _, informer := range w.informers {
		w.wg.Add(1)
		go func(informer cache.SharedInformer) {
			defer w.wg.Done()
			informer.Run(w.stopCh)
		}(informer)
	}

	slog.Info("Kubernetes watcher started successfully")
	return nil
}

// Stop stops the watcher
func (w *Watcher) Stop() {
	slog.Info("Stopping Kubernetes watcher")
	close(w.stopCh)

	// Stop audit log handler if enabled
	if w.auditLogHandler != nil {
		w.auditLogHandler.Stop()
	}

	// Stop CloudWatch handler if enabled
	if w.cloudwatchHandler != nil {
		w.cloudwatchHandler.Stop()
	}

	// Wait for all goroutines to finish
	w.wg.Wait()

	w.mu.Lock()
	defer w.mu.Unlock()

	// Stop all watchers
	for resourceType, watcher := range w.watchers {
		watcher.Stop()
		slog.Info("Stopped watching resource", "resource", resourceType)
	}

	close(w.eventChannel)
	slog.Info("Kubernetes watcher stopped")
}

// getResourcesToWatch returns the list of resources to watch
func (w *Watcher) getResourcesToWatch() []string {
	// Watch resources needed for policy violation detection
	return []string{
		"events",
		"PolicyReport",
		"ClusterPolicyReport",
		"Policy",
	}
}

func (w *Watcher) watchResource(ctx context.Context, resourceType string) error {
	resourceInterface, err := w.client.WatchResources(ctx, resourceType)
	if err != nil {
		return fmt.Errorf("failed to get resource interface for %s: %w", resourceType, err)
	}

	watcher, err := resourceInterface.Watch(ctx, v1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to start watcher for %s: %w", resourceType, err)
	}

	w.mu.Lock()
	w.watchers[resourceType] = watcher
	w.mu.Unlock()

	// Start watching for events
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.watchEvents(resourceType, watcher)
	}()

	return nil
}

func (w *Watcher) watchEvents(resourceType string, watcher watch.Interface) {
	defer watcher.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				slog.Warn("Watcher channel closed, attempting to reconnect", "resource", resourceType)
				// Attempt to reconnect
				if err := w.reconnectWatcher(resourceType); err != nil {
					slog.Error("Failed to reconnect watcher", "error", err, "resource", resourceType)
					return
				}
				slog.Info("Successfully reconnected watcher", "resource", resourceType)
				return // Exit this goroutine, new one will be started by reconnectWatcher
			}

			if err := w.handleEvent(resourceType, event); err != nil {
				slog.Error("Failed to handle event - this may indicate issues with event processing or resource parsing",
					"error", err,
					"resource", resourceType,
					"event_type", event.Type,
					"error_type", fmt.Sprintf("%T", err))
			}
		}
	}
}

func (w *Watcher) handleEvent(resourceType string, kubeEvent watch.Event) error {
	var eventType event.EventType

	switch kubeEvent.Type {
	case watch.Added:
		eventType = event.EventTypeAdded
	case watch.Modified:
		eventType = event.EventTypeModified
	case watch.Deleted:
		eventType = event.EventTypeDeleted
	case watch.Error:
		eventType = event.EventTypeError
	default:
		return fmt.Errorf("unknown event type: %s", kubeEvent.Type)
	}

	watchedEvent, err := event.NewWatchedEvent(eventType, kubeEvent.Object, resourceType)
	if err != nil {
		return fmt.Errorf("failed to create watched event: %w", err)
	}

	// Record event being added to channel
	w.metrics.RecordEventInChannel()

	// Try to send event with backpressure handling
	if err := w.sendEventWithBackpressure(watchedEvent, resourceType, eventType); err != nil {
		// If all retries failed, record as dropped
		w.metrics.RecordEventDropped()
		slog.Warn("Failed to queue event after retries - event dropped",
			"error", err,
			"resource_type", resourceType,
			"event_type", eventType,
			"namespace", watchedEvent.Namespace,
			"name", watchedEvent.Name)
	}

	return nil
}

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

// sendEventWithBackpressure attempts to send an event to the channel with retry logic
func (w *Watcher) sendEventWithBackpressure(watchedEvent *event.WatchedEvent, resourceType string, eventType event.EventType) error {
	for attempt := 0; attempt <= w.backpressureConfig.MaxRetries; attempt++ {
		select {
		case w.eventChannel <- watchedEvent:
			// Event successfully queued
			return nil
		case <-w.stopCh:
			// Watcher is stopping, don't retry
			return fmt.Errorf("watcher is stopping")
		default:
			// Channel is full, retry if we haven't exceeded max retries
			if attempt < w.backpressureConfig.MaxRetries {
				slog.Debug("Event channel full, retrying...",
					"resource_type", resourceType,
					"event_type", eventType,
					"namespace", watchedEvent.Namespace,
					"name", watchedEvent.Name,
					"attempt", attempt+1,
					"max_retries", w.backpressureConfig.MaxRetries,
					"retry_delay", w.backpressureConfig.RetryDelay)

				// Wait before retrying
				time.Sleep(w.backpressureConfig.RetryDelay)
			}
		}
	}

	return fmt.Errorf("failed to queue event after %d retries", w.backpressureConfig.MaxRetries)
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

// GetMetrics returns the current metrics for external monitoring
func (w *Watcher) GetMetrics() *metrics.Metrics {
	return w.metrics
}

// reconnectWatcher attempts to reconnect a watcher for a specific resource type
func (w *Watcher) reconnectWatcher(resourceType string) error {
	const maxRetries = 5
	const retryDelay = 5 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Check if we should stop
		select {
		case <-w.stopCh:
			return fmt.Errorf("watcher is stopping")
		default:
		}

		slog.Info("Attempting to reconnect watcher",
			"resource", resourceType,
			"attempt", attempt,
			"max_retries", maxRetries)

		// Create new context for the reconnection attempt
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		// Try to reconnect
		if err := w.watchResource(ctx, resourceType); err != nil {
			cancel()
			slog.Warn("Failed to reconnect watcher, retrying...",
				"error", err,
				"resource", resourceType,
				"attempt", attempt)

			if attempt < maxRetries {
				// Wait before retrying
				select {
				case <-w.stopCh:
					return fmt.Errorf("watcher is stopping")
				case <-time.After(retryDelay):
					continue
				}
			}
			return fmt.Errorf("failed to reconnect watcher after %d attempts: %w", maxRetries, err)
		}

		cancel()
		slog.Info("Successfully reconnected watcher", "resource", resourceType)
		return nil
	}

	return fmt.Errorf("failed to reconnect watcher after %d attempts", maxRetries)
}

// checkExistingPolicies checks existing policies for audit duplicates
func (w *Watcher) checkExistingPolicies() error {
	// No longer checking ClusterPolicies for audit duplicates
	slog.Debug("ClusterPolicy duplicator logic removed, skipping existing policy check")
	return nil
}

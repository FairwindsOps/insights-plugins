package watcher

import (
	"fmt"
	"log/slog"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/client"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
)

// EventSourceType represents the type of event source
type EventSourceType string

const (
	EventSourceTypeAuditLog   EventSourceType = "audit-log"
	EventSourceTypeCloudWatch EventSourceType = "cloudwatch"
)

// EventSourceConfig represents configuration for creating event sources
type EventSourceConfig struct {
	Type           EventSourceType
	InsightsConfig models.InsightsConfig
	KubeClient     *client.Client
	EventChannel   chan *event.WatchedEvent

	// Source-specific configurations
	AuditLogPath     string
	CloudWatchConfig *models.CloudWatchConfig
	FileSystemPath   string
	WebhookURL       string
	DatabaseConfig   map[string]interface{}
}

// EventSourceFactory creates event sources based on configuration
type EventSourceFactory struct {
	creators map[EventSourceType]EventSourceCreator
}

// EventSourceCreator is a function that creates an event source
type EventSourceCreator func(config EventSourceConfig) (EventSource, error)

// NewEventSourceFactory creates a new event source factory
func NewEventSourceFactory() *EventSourceFactory {
	factory := &EventSourceFactory{
		creators: make(map[EventSourceType]EventSourceCreator),
	}

	// Register default creators
	factory.registerDefaultCreators()

	return factory
}

// registerDefaultCreators registers all default event source creators
func (f *EventSourceFactory) registerDefaultCreators() {
	f.RegisterCreator(EventSourceTypeAuditLog, f.createAuditLogEventSource)
	f.RegisterCreator(EventSourceTypeCloudWatch, f.createCloudWatchEventSource)
}

// RegisterCreator registers a new event source creator
func (f *EventSourceFactory) RegisterCreator(sourceType EventSourceType, creator EventSourceCreator) {
	f.creators[sourceType] = creator
	slog.Debug("Registered event source creator", "type", sourceType)
}

// CreateEventSource creates an event source based on the configuration
func (f *EventSourceFactory) CreateEventSource(config EventSourceConfig) (EventSource, error) {
	creator, exists := f.creators[config.Type]
	if !exists {
		return nil, fmt.Errorf("unsupported event source type: %s", config.Type)
	}

	slog.Debug("Creating event source", "type", config.Type)
	return creator(config)
}

// CreateEventSources creates multiple event sources based on a list of configurations
func (f *EventSourceFactory) CreateEventSources(configs []EventSourceConfig) ([]EventSource, error) {
	var sources []EventSource
	var errors []error

	for _, config := range configs {
		source, err := f.CreateEventSource(config)
		if err != nil {
			slog.Error("Failed to create event source", "type", config.Type, "error", err)
			errors = append(errors, fmt.Errorf("failed to create %s event source: %w", config.Type, err))
			continue
		}

		sources = append(sources, source)
		slog.Info("Created event source", "type", config.Type, "enabled", source.IsEnabled())
	}

	if len(errors) > 0 {
		return sources, fmt.Errorf("failed to create %d event sources: %v", len(errors), errors)
	}

	return sources, nil
}

// GetSupportedTypes returns all supported event source types
func (f *EventSourceFactory) GetSupportedTypes() []EventSourceType {
	types := make([]EventSourceType, 0, len(f.creators))
	for sourceType := range f.creators {
		types = append(types, sourceType)
	}
	return types
}

// createAuditLogEventSource creates an audit log event source
func (f *EventSourceFactory) createAuditLogEventSource(config EventSourceConfig) (EventSource, error) {
	if config.AuditLogPath == "" {
		return nil, fmt.Errorf("audit log path is required for audit log event source")
	}

	if config.KubeClient == nil {
		return nil, fmt.Errorf("kubernetes client is required for audit log event source")
	}

	return NewAuditLogEventSourceAdapter(
		config.InsightsConfig,
		config.KubeClient.KubeInterface,
		config.AuditLogPath,
		config.EventChannel,
	), nil
}

// createCloudWatchEventSource creates a CloudWatch event source
func (f *EventSourceFactory) createCloudWatchEventSource(config EventSourceConfig) (EventSource, error) {
	if config.CloudWatchConfig == nil {
		return nil, fmt.Errorf("cloudwatch config is required for cloudwatch event source")
	}

	return NewCloudWatchEventSourceAdapter(
		config.InsightsConfig,
		*config.CloudWatchConfig,
		config.EventChannel,
	)
}

// BuildEventSourceConfigs creates a list of event source configurations based on the watcher parameters
func BuildEventSourceConfigs(insightsConfig models.InsightsConfig, kubeClient *client.Client, logSource, auditLogPath string, cloudwatchConfig *models.CloudWatchConfig, eventChannel chan *event.WatchedEvent) []EventSourceConfig {
	var configs []EventSourceConfig

	// Add audit log event source if enabled (for local/kind clusters)
	if auditLogPath != "" {
		configs = append(configs, EventSourceConfig{
			Type:           EventSourceTypeAuditLog,
			InsightsConfig: insightsConfig,
			KubeClient:     kubeClient,
			EventChannel:   eventChannel,
			AuditLogPath:   auditLogPath,
		})
	}

	// Add CloudWatch event source if enabled (for EKS clusters)
	if logSource == "cloudwatch" && cloudwatchConfig != nil {
		configs = append(configs, EventSourceConfig{
			Type:             EventSourceTypeCloudWatch,
			InsightsConfig:   insightsConfig,
			KubeClient:       kubeClient,
			EventChannel:     eventChannel,
			CloudWatchConfig: cloudwatchConfig,
		})
	}

	return configs
}

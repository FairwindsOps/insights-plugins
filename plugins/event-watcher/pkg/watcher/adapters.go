package watcher

import (
	"context"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/handlers"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
	"k8s.io/client-go/kubernetes"
)

// AuditLogEventSourceAdapter adapts AuditLogHandler to implement EventSource interface
type AuditLogEventSourceAdapter struct {
	handler *handlers.AuditLogHandler
	enabled bool
}

// NewAuditLogEventSourceAdapter creates a new adapter for audit log handler
func NewAuditLogEventSourceAdapter(config models.InsightsConfig, kubeClient kubernetes.Interface, auditLogPath string, eventChannel chan *models.WatchedEvent) *AuditLogEventSourceAdapter {
	handler := handlers.NewAuditLogHandler(config, kubeClient, auditLogPath, eventChannel)

	return &AuditLogEventSourceAdapter{
		handler: handler,
		enabled: auditLogPath != "",
	}
}

// Start implements EventSource interface
func (a *AuditLogEventSourceAdapter) Start(ctx context.Context) error {
	return a.handler.Start(ctx)
}

// Stop implements EventSource interface
func (a *AuditLogEventSourceAdapter) Stop() {
	a.handler.Stop()
}

// GetName implements EventSource interface
func (a *AuditLogEventSourceAdapter) GetName() string {
	return "audit-log"
}

// IsEnabled implements EventSource interface
func (a *AuditLogEventSourceAdapter) IsEnabled() bool {
	return a.enabled
}

// CloudWatchEventSourceAdapter adapts CloudWatchHandler to implement EventSource interface
type CloudWatchEventSourceAdapter struct {
	handler *handlers.CloudWatchHandler
	enabled bool
}

// NewCloudWatchEventSourceAdapter creates a new adapter for CloudWatch handler
func NewCloudWatchEventSourceAdapter(config models.InsightsConfig, cloudwatchConfig models.CloudWatchConfig, eventChannel chan *models.WatchedEvent) (*CloudWatchEventSourceAdapter, error) {
	handler, err := handlers.NewCloudWatchHandler(config, cloudwatchConfig, eventChannel)
	if err != nil {
		return nil, err
	}

	return &CloudWatchEventSourceAdapter{
		handler: handler,
		enabled: true, // CloudWatch is enabled if we can create the handler
	}, nil
}

// Start implements EventSource interface
func (a *CloudWatchEventSourceAdapter) Start(ctx context.Context) error {
	return a.handler.Start(ctx)
}

// Stop implements EventSource interface
func (a *CloudWatchEventSourceAdapter) Stop() {
	a.handler.Stop()
}

// GetName implements EventSource interface
func (a *CloudWatchEventSourceAdapter) GetName() string {
	return "cloudwatch"
}

// IsEnabled implements EventSource interface
func (a *CloudWatchEventSourceAdapter) IsEnabled() bool {
	return a.enabled
}

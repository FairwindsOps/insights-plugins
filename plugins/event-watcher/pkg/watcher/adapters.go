package watcher

import (
	"context"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/client"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/producers"
	"k8s.io/client-go/kubernetes"
)

// AuditLogEventSourceAdapter adapts AuditLogHandler to implement EventSource interface
type AuditLogEventSourceAdapter struct {
	producer          *producers.AuditLogHandler
	enabled           bool
	eventPollInterval string
}

// NewAuditLogEventSourceAdapter creates a new adapter for audit log handler
func NewAuditLogEventSourceAdapter(config models.InsightsConfig, kubeClient kubernetes.Interface, auditLogPath string, eventChannel chan *models.WatchedEvent, eventPollInterval string) *AuditLogEventSourceAdapter {
	producer := producers.NewAuditLogHandler(config, kubeClient, auditLogPath, eventChannel)

	return &AuditLogEventSourceAdapter{
		producer: producer,
		enabled:  auditLogPath != "",
	}
}

// Start implements EventSource interface
func (a *AuditLogEventSourceAdapter) Start(ctx context.Context) error {
	return a.producer.Start(ctx)
}

// Stop implements EventSource interface
func (a *AuditLogEventSourceAdapter) Stop() {
	a.producer.Stop()
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
	producer *producers.CloudWatchHandler
	enabled  bool
}

// NewCloudWatchEventSourceAdapter creates a new adapter for CloudWatch handler
func NewCloudWatchEventSourceAdapter(config models.InsightsConfig, cloudwatchConfig models.CloudWatchConfig, eventChannel chan *models.WatchedEvent) (*CloudWatchEventSourceAdapter, error) {
	producer, err := producers.NewCloudWatchHandler(config, cloudwatchConfig, eventChannel)
	if err != nil {
		return nil, err
	}

	return &CloudWatchEventSourceAdapter{
		producer: producer,
		enabled:  true, // CloudWatch is enabled if we can create the handler
	}, nil
}

// Start implements EventSource interface
func (a *CloudWatchEventSourceAdapter) Start(ctx context.Context) error {
	return a.producer.Start(ctx)
}

// Stop implements EventSource interface
func (a *CloudWatchEventSourceAdapter) Stop() {
	a.producer.Stop()
}

// GetName implements EventSource interface
func (a *CloudWatchEventSourceAdapter) GetName() string {
	return "cloudwatch"
}

// IsEnabled implements EventSource interface
func (a *CloudWatchEventSourceAdapter) IsEnabled() bool {
	return a.enabled
}

type KubernetesEventSourceAdapter struct {
	producer     *producers.KubernetesEventHandler
	pollInterval string
}

// NewKubernetesEventSourceAdapter creates a new adapter for Kubernetes handler
func NewKubernetesEventSourceAdapter(config models.InsightsConfig, kubeClient *client.Client, pollInterval string, eventChannel chan *models.WatchedEvent) *KubernetesEventSourceAdapter {
	producer := producers.NewKubernetesEventHandler(config, kubeClient, pollInterval, eventChannel)
	return &KubernetesEventSourceAdapter{
		producer:     producer,
		pollInterval: pollInterval,
	}
}

// Start implements EventSource interface
func (a *KubernetesEventSourceAdapter) Start(ctx context.Context) error {
	return a.producer.Start(ctx)
}

// Stop implements EventSource interface
func (a *KubernetesEventSourceAdapter) Stop() {
	a.producer.Stop()
}

// GetName implements EventSource interface
func (a *KubernetesEventSourceAdapter) GetName() string {
	return "kubernetes"
}

// IsEnabled implements EventSource interface
func (a *KubernetesEventSourceAdapter) IsEnabled() bool {
	return true
}

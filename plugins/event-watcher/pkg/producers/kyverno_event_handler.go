package producers

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/client"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	EventVersion = 1
)

type KubernetesEventHandler struct {
	eventChannel chan *models.WatchedEvent
	kubeClient   *client.Client
	pollInterval string
	stopCh       chan struct{}
}

// NewKubernetesEventHandler creates a new KubernetesEventHandler
func NewKubernetesEventHandler(insightsConfig models.InsightsConfig, kubeClient *client.Client, pollInterval string, eventChannel chan *models.WatchedEvent) *KubernetesEventHandler {
	return &KubernetesEventHandler{
		eventChannel: eventChannel,
		kubeClient:   kubeClient,
		pollInterval: pollInterval,
	}
}

// Start begins processing CloudWatch logs
func (h *KubernetesEventHandler) Start(ctx context.Context) error {
	slog.Info("Starting Kubernetes event processing")

	// Parse poll interval
	pollInterval, err := time.ParseDuration(h.pollInterval)
	if err != nil {
		return fmt.Errorf("invalid poll interval '%s': %w", pollInterval, err)
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Kubernetes event processing context cancelled")
			return ctx.Err()
		case <-ticker.C:
			if err := h.processKubernetesEvents(ctx); err != nil {
				slog.Error("Failed to process events", "error", err)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

// Stop stops the CloudWatch handler
func (h *KubernetesEventHandler) Stop() {
	close(h.stopCh)
}

// NewWatchedEvent creates a new WatchedEvent from a Kubernetes object
func (h *KubernetesEventHandler) processKubernetesEvents(ctx context.Context) error {
	events, err := h.kubeClient.KubeInterface.CoreV1().Events("").List(ctx, metav1.ListOptions{
		//FieldSelector: "involvedObject.name=kyverno-policy-report",
	})
	slog.Info("Processing Kubernetes events", "events", len(events.Items))
	if err != nil {
		return fmt.Errorf("failed to list latest kubernetes events: %w", err)
	}
	for _, event := range events.Items {
		event := &models.WatchedEvent{
			EventVersion: EventVersion,
			Timestamp:    time.Now().Unix(),
			EventTime:    event.LastTimestamp.Format(time.RFC3339),
			EventType:    models.EventType(event.Type),
			ResourceType: event.InvolvedObject.Kind,
			Namespace:    event.InvolvedObject.Namespace,
			Name:         event.InvolvedObject.Name,
			UID:          string(event.InvolvedObject.UID),
			Data:         map[string]interface{}{"event": event},
			Metadata:     map[string]interface{}{"annotations": event.ObjectMeta.Annotations, "labels": event.ObjectMeta.Labels},
			EventSource:  "kubernetes",
			Success:      false,
			Blocked:      true, // TODO: Fix this
		}
		h.eventChannel <- event
	}
	return nil
}

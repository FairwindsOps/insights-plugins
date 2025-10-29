package producers

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	EventVersion = 1
)

type KubernetesEventHandler struct {
	eventChannel chan *models.WatchedEvent
	pollInterval string
	stopCh       chan struct{}
}

// NewKubernetesEventHandler creates a new KubernetesEventHandler
func NewKubernetesEventHandler(insightsConfig models.InsightsConfig, eventChannel chan *models.WatchedEvent) *KubernetesEventHandler {
	return &KubernetesEventHandler{
		eventChannel: eventChannel,
		//pollInterval: pollInterval,
	}
}

// NewWatchedEvent creates a new WatchedEvent from a Kubernetes object
func (h *KubernetesEventHandler) processKubernetesEvent(eventType models.EventType, obj interface{}, resourceType string) (*models.WatchedEvent, error) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		// Try to convert from runtime.Object
		if runtimeObj, ok := obj.(runtime.Object); ok {
			unstructuredObj = &unstructured.Unstructured{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(runtimeObj.(*unstructured.Unstructured).Object, unstructuredObj); err != nil {
				slog.Warn("failed to convert object to unstructured", "error", err)
				return nil, fmt.Errorf("failed to convert object to unstructured: %w", err)
			}
		} else {
			slog.Warn("unable to convert object to unstructured")
			return nil, fmt.Errorf("unable to convert object to unstructured")
		}
	}

	metadataRaw, exists := unstructuredObj.Object["metadata"]
	if !exists {
		return nil, fmt.Errorf("object missing metadata field")
	}
	metadata, ok := metadataRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("metadata is not a map")
	}

	name := ""
	namespace := ""
	uid := ""

	if nameVal, ok := metadata["name"].(string); ok {
		name = nameVal
	}
	if namespaceVal, ok := metadata["namespace"].(string); ok {
		namespace = namespaceVal
	}
	if uidVal, ok := metadata["uid"].(string); ok {
		uid = uidVal
	}

	// Remove managedFields from metadata for cleaner output
	delete(metadata, "managedFields")

	// Extract Kubernetes eventTime if available
	var eventTime time.Time
	// try to convert eventTime to a time.Time object
	if eventTimeVal, ok := unstructuredObj.Object["eventTime"].(string); ok {
		eventTimeTime, err := time.Parse(time.RFC3339, eventTimeVal)
		if err != nil {
			slog.Warn("failed to parse eventTime", "error", err)
		} else {
			eventTime = eventTimeTime
		}
	}
	if eventTime.IsZero() {
		slog.Warn("eventTime is zero")
		eventTime = time.Now().UTC()
	}
	eventTimeString := eventTime.UTC().Format(time.RFC3339)

	event := &models.WatchedEvent{
		EventVersion: EventVersion,
		Timestamp:    time.Now().Unix(), // Processing timestamp
		EventTime:    eventTimeString,   // Kubernetes eventTime
		EventType:    eventType,
		ResourceType: resourceType,
		Namespace:    namespace,
		Name:         name,
		UID:          uid,
		Data:         unstructuredObj.Object,
		Metadata:     metadata,
		EventSource:  "kubernetes",
	}

	return event, nil
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
			if err := h.processEvents(ctx); err != nil {
				slog.Error("Failed to process events", "error", err)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

// processEvents processes events from the event channel
func (h *KubernetesEventHandler) processEvents(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			slog.Info("Kubernetes event processing context cancelled")
			return ctx.Err()
		case obj, ok := <-h.eventChannel:
			if !ok {
				slog.Debug("Event channel closed, stopping event processing")
				return nil
			}
			event, err := h.processKubernetesEvent(models.EventTypeAdded, obj, "events")
			if err != nil {
				slog.Error("Failed to process Kubernetes event", "error", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			if event != nil {
				select {
				case h.eventChannel <- event:
					slog.Debug("Sent event to channel", "event", event)
				case <-ctx.Done():
					return ctx.Err()
				case <-h.stopCh:
					slog.Info("Kubernetes event processing stopped")
					return nil
				default:
					slog.Warn("Event channel full, dropping event")
				}
			}
			return nil
		}
	}
}

// Stop stops the CloudWatch handler
func (h *KubernetesEventHandler) Stop() {
	close(h.stopCh)
}

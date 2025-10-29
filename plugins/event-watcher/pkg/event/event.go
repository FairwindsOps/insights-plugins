package event

import (
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

// NewWatchedEvent creates a new WatchedEvent from a Kubernetes object
func NewWatchedEvent(eventType models.EventType, obj interface{}, resourceType string) (*models.WatchedEvent, error) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		// Try to convert from runtime.Object
		if runtimeObj, ok := obj.(runtime.Object); ok {
			unstructuredObj = &unstructured.Unstructured{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(runtimeObj.(*unstructured.Unstructured).Object, unstructuredObj); err != nil {
				return nil, err
			}
		} else {
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
	}

	return event, nil
}

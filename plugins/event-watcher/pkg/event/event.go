package event

import (
	"encoding/json"
	"fmt"
	"time"

	"log/slog"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	EventVersion = 1
)

// EventType represents the type of Kubernetes event
type EventType string

const (
	EventTypeAdded    EventType = "ADDED"
	EventTypeModified EventType = "MODIFIED"
	EventTypeDeleted  EventType = "DELETED"
	EventTypeError    EventType = "ERROR"
)

// WatchedEvent represents a Kubernetes event that we're watching
type WatchedEvent struct {
	EventVersion int                    `json:"event_version"`
	Timestamp    int64                  `json:"timestamp"`            // Processing timestamp
	EventTime    string                 `json:"event_time,omitempty"` // Kubernetes eventTime
	EventType    EventType              `json:"event_type"`
	ResourceType string                 `json:"resource_type"`
	Namespace    string                 `json:"namespace"`
	Name         string                 `json:"name"`
	UID          string                 `json:"uid"`
	Data         map[string]interface{} `json:"data"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// NewWatchedEvent creates a new WatchedEvent from a Kubernetes object
func NewWatchedEvent(eventType EventType, obj interface{}, resourceType string) (*WatchedEvent, error) {
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
	var eventTime string
	if eventTimeVal, ok := unstructuredObj.Object["eventTime"].(string); ok {
		eventTime = eventTimeVal
	} else {
		eventTime = time.Now().UTC().Format(time.RFC3339)
	}

	event := &WatchedEvent{
		EventVersion: EventVersion,
		Timestamp:    time.Now().Unix(), // Processing timestamp
		EventTime:    eventTime,         // Kubernetes eventTime
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

// ToJSON converts the event to JSON bytes
func (e *WatchedEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// LogEvent logs the event with appropriate level
func (e *WatchedEvent) LogEvent() {
	fields := []interface{}{
		"event_type", e.EventType,
		"resource_type", e.ResourceType,
		"namespace", e.Namespace,
		"name", e.Name,
		"uid", e.UID,
		"timestamp", e.Timestamp,
	}

	switch e.EventType {
	case EventTypeAdded:
		slog.Info("Resource added", fields...)
	case EventTypeModified:
		slog.Info("Resource modified", fields...)
	case EventTypeDeleted:
		slog.Info("Resource deleted", fields...)
	case EventTypeError:
		slog.Error("Resource event error", fields...)
	default:
		slog.Info("Resource event", fields...)
	}
}

// IsKyvernoResource checks if the event is related to Kyverno
func (e *WatchedEvent) IsKyvernoResource() bool {
	kyvernoResources := []string{
		"PolicyReport",
		"ClusterPolicyReport",
		"Policy",
		"ClusterPolicy",
		"ValidatingAdmissionPolicy",
		"ValidatingAdmissionPolicyBinding",
		"MutatingAdmissionPolicy",
		"MutatingAdmissionPolicyBinding",
	}

	for _, resource := range kyvernoResources {
		if e.ResourceType == resource {
			return true
		}
	}
	return false
}

// GetPolicyName extracts policy name from Kyverno events
func (e *WatchedEvent) GetPolicyName() string {
	if !e.IsKyvernoResource() {
		return ""
	}

	// For PolicyReport and ClusterPolicyReport, look in the results
	if e.ResourceType == "PolicyReport" || e.ResourceType == "ClusterPolicyReport" {
		if results, ok := e.Data["results"].([]interface{}); ok {
			for _, result := range results {
				if resultMap, ok := result.(map[string]interface{}); ok {
					if policy, ok := resultMap["policy"].(string); ok {
						return policy
					}
				}
			}
		}
	}

	// For other resources, use the name
	return e.Name
}

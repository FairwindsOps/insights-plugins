package event

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
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
	Timestamp    int64                  `json:"timestamp"`
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

	metadata := unstructuredObj.Object["metadata"].(map[string]interface{})

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

	event := &WatchedEvent{
		EventVersion: EventVersion,
		Timestamp:    time.Now().Unix(),
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
	fields := logrus.Fields{
		"event_type":    e.EventType,
		"resource_type": e.ResourceType,
		"namespace":     e.Namespace,
		"name":          e.Name,
		"uid":           e.UID,
		"timestamp":     e.Timestamp,
	}

	switch e.EventType {
	case EventTypeAdded:
		logrus.WithFields(fields).Info("Resource added")
	case EventTypeModified:
		logrus.WithFields(fields).Info("Resource modified")
	case EventTypeDeleted:
		logrus.WithFields(fields).Info("Resource deleted")
	case EventTypeError:
		logrus.WithFields(fields).Error("Resource event error")
	default:
		logrus.WithFields(fields).Info("Resource event")
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

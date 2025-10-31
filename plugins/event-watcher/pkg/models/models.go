package models

import (
	"encoding/json"
	"log/slog"
	"time"
)

type InsightsConfig struct {
	Hostname     string
	Organization string
	Cluster      string
	Token        string
}

// ViolationEvent represents a detected policy violation from audit logs
type PolicyViolationEventModel struct {
	Timestamp    time.Time                    `json:"timestamp"`
	ResourceType string                       `json:"resource_type"`
	ResourceName string                       `json:"resource_name"`
	Namespace    string                       `json:"namespace"`
	User         string                       `json:"user"`
	Action       string                       `json:"action"` // "blocked" or "allowed"
	Message      string                       `json:"message"`
	AuditID      string                       `json:"audit_id"`
	Metadata     map[string]interface{}       `json:"metadata"`
	Policies     map[string]map[string]string `json:"policies"`
}

// AuditEvent represents a Kubernetes audit log entry
type AuditEvent struct {
	Kind                     string            `json:"kind"`
	APIVersion               string            `json:"apiVersion"`
	Level                    string            `json:"level"`
	AuditID                  string            `json:"auditID"`
	Stage                    string            `json:"stage"`
	RequestURI               string            `json:"requestURI"`
	Verb                     string            `json:"verb"`
	User                     User              `json:"user"`
	SourceIPs                []string          `json:"sourceIPs"`
	UserAgent                string            `json:"userAgent"`
	ObjectRef                ObjectRef         `json:"objectRef"`
	ResponseStatus           ResponseStatus    `json:"responseStatus"`
	RequestObject            interface{}       `json:"requestObject"`
	ResponseObject           interface{}       `json:"responseObject"`
	Annotations              map[string]string `json:"annotations"`
	RequestReceivedTimestamp time.Time         `json:"requestReceivedTimestamp"`
	StageTimestamp           time.Time         `json:"stageTimestamp"`
}

type User struct {
	Username string   `json:"username"`
	UID      string   `json:"uid"`
	Groups   []string `json:"groups"`
}

type ObjectRef struct {
	Resource        string `json:"resource"`
	Namespace       string `json:"namespace"`
	Name            string `json:"name"`
	UID             string `json:"uid"`
	APIGroup        string `json:"apiGroup"`
	APIVersion      string `json:"apiVersion"`
	ResourceVersion string `json:"resourceVersion"`
	SubResource     string `json:"subResource"`
}

type ResponseStatus struct {
	Metadata map[string]interface{} `json:"metadata"`
	Code     int                    `json:"code"`
	Status   string                 `json:"status"`
	Message  string                 `json:"message"`
	Reason   string                 `json:"reason"`
}

type CloudWatchConfig struct {
	LogGroupName  string
	Region        string
	FilterPattern string
	BatchSize     int
	PollInterval  string
	MaxMemoryMB   int
}
type EventReport struct {
	EventType    string                 `json:"eventType"`
	ResourceType string                 `json:"resourceType"`
	Namespace    string                 `json:"namespace"`
	Name         string                 `json:"name"`
	UID          string                 `json:"uid"`
	Timestamp    int64                  `json:"timestamp"`
	Data         map[string]interface{} `json:"data"`
	Metadata     map[string]interface{} `json:"metadata"`
}

type PolicyViolationEvent struct {
	EventReport
	Policies  map[string]map[string]string `json:"policies"`
	Message   string                       `json:"message"`
	Blocked   bool                         `json:"blocked"`
	Success   bool                         `json:"success"`
	EventTime string                       `json:"eventTime,omitempty"` // Kubernetes eventTime
}

type EventHandlerConfig struct {
	Enabled     bool              `json:"enabled"`
	EventTypes  []string          `json:"event_types"`
	Filters     map[string]string `json:"filters"`
	InsightsAPI bool              `json:"insights_api"`
}

type WatcherConfig struct {
	Insights      InsightsConfig                `json:"insights"`
	EventHandlers map[string]EventHandlerConfig `json:"event_handlers"`
	KyvernoOnly   bool                          `json:"kyverno_only"`
	LogLevel      string                        `json:"log_level"`
}

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
	EventSource  string                 `json:"event_source"`
	Success      bool                   `json:"success"`
	Blocked      bool                   `json:"blocked"`
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
		"success", e.Success,
		"blocked", e.Blocked,
	}

	switch e.EventType {
	case EventTypeAdded:
		slog.Debug("Resource added", fields...)
	case EventTypeModified:
		slog.Debug("Resource modified", fields...)
	case EventTypeDeleted:
		slog.Debug("Resource deleted", fields...)
	case EventTypeError:
		slog.Error("Resource event error", fields...)
	default:
		slog.Debug("Resource event", fields...)
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

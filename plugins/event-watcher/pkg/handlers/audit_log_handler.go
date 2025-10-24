package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"log/slog"

	"k8s.io/client-go/kubernetes"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
)

// AuditLogHandler handles audit log monitoring and policy violation detection
type AuditLogHandler struct {
	insightsConfig models.InsightsConfig
	kubeClient     kubernetes.Interface
	auditLogPath   string
	eventChannel   chan *event.WatchedEvent
	stopCh         chan struct{}
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

// NewAuditLogHandler creates a new audit log handler
func NewAuditLogHandler(config models.InsightsConfig, kubeClient kubernetes.Interface, auditLogPath string, eventChannel chan *event.WatchedEvent) *AuditLogHandler {
	return &AuditLogHandler{
		insightsConfig: config,
		kubeClient:     kubeClient,
		auditLogPath:   auditLogPath,
		eventChannel:   eventChannel,
		stopCh:         make(chan struct{}),
	}
}

// Start begins monitoring the audit log file
func (h *AuditLogHandler) Start(ctx context.Context) error {
	slog.Info("Starting audit log monitoring", "audit_log_path", h.auditLogPath)

	// Check if audit log file exists
	if _, err := os.Stat(h.auditLogPath); os.IsNotExist(err) {
		slog.Warn("Audit log file does not exist, audit log monitoring disabled", "audit_log_path", h.auditLogPath)
		return nil
	}

	// Start monitoring the audit log file
	go h.monitorAuditLog(ctx)
	return nil
}

// Stop stops monitoring the audit log file
func (h *AuditLogHandler) Stop() {
	close(h.stopCh)
}

// monitorAuditLog continuously monitors the audit log file for new entries
func (h *AuditLogHandler) monitorAuditLog(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.processNewAuditLogEntries()
		}
	}
}

// processNewAuditLogEntries processes new entries in the audit log file
func (h *AuditLogHandler) processNewAuditLogEntries() {
	file, err := os.Open(h.auditLogPath)
	if err != nil {
		slog.Error("Failed to open audit log file", "error", err, "audit_log_path", h.auditLogPath)
		return
	}
	defer file.Close()

	// Get file info to track position
	fileInfo, err := file.Stat()
	if err != nil {
		slog.Error("Failed to get file info", "error", err)
		return
	}

	// For simplicity, we'll process the entire file each time
	// In production, you'd want to track file position to avoid reprocessing
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse the audit log entry
		var auditEvent AuditEvent
		if err := json.Unmarshal([]byte(line), &auditEvent); err != nil {
			slog.Debug("Failed to parse audit log line",
				"error", err,
				"line_number", lineNumber,
				"audit_log_path", h.auditLogPath)
			continue
		}

		if !h.isKyvernoPolicyViolation(auditEvent) {
			continue
		}
		policyViolationEvent := h.createPolicyViolationEvent(auditEvent)
		if policyViolationEvent != nil {
			h.createWatchedEventFromPolicyViolationEvent(policyViolationEvent)
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error("Error reading audit log file", "error", err, "audit_log_path", h.auditLogPath)
	}

	slog.Debug("Processed audit log entries",
		"file_size", fileInfo.Size(),
		"lines_processed", lineNumber)
}

func (h *AuditLogHandler) isKyvernoPolicyViolation(auditEvent AuditEvent) bool {
	message := auditEvent.ResponseStatus.Message
	if auditEvent.ResponseStatus.Code >= 400 && strings.Contains(message, "kyverno") &&
		strings.Contains(message, "blocked due to the following policies") {
		return true
	}
	return false
}

// createPolicyViolation creates a policy violation event from an audit event
func (h *AuditLogHandler) createPolicyViolationEvent(auditEvent AuditEvent) *PolicyViolationEvent {
	if !h.isKyvernoPolicyViolation(auditEvent) {
		return nil
	}

	// This is a blocked request - extract policy violation information
	policies := ExtractPoliciesFromMessage(auditEvent.ResponseStatus.Message)

	slog.Info("Detected policy violation from audit logs",
		"audit_id", auditEvent.AuditID,
		"policies", policies,
		"resource_name", auditEvent.ObjectRef.Name,
		"namespace", auditEvent.ObjectRef.Namespace,
		"response_code", auditEvent.ResponseStatus.Code,
		"message", auditEvent.ResponseStatus.Message,
		"level", auditEvent.Level,
		"stage", auditEvent.Stage)

	return &PolicyViolationEvent{
		Timestamp:    auditEvent.RequestReceivedTimestamp,
		Policies:     policies,
		ResourceType: auditEvent.ObjectRef.Resource,
		ResourceName: auditEvent.ObjectRef.Name,
		Namespace:    auditEvent.ObjectRef.Namespace,
		User:         auditEvent.User.Username,
		Action:       auditEvent.ResponseStatus.Status,
		Message:      auditEvent.ResponseStatus.Message,
		AuditID:      auditEvent.AuditID,
	}
}

// PolicyViolationEvent represents a detected policy violation from audit logs
type PolicyViolationEvent struct {
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

// createWatchedEventFromPolicyViolationEvent creates a watched event from a policy violation event
func (h *AuditLogHandler) createWatchedEventFromPolicyViolationEvent(violation *PolicyViolationEvent) {
	if violation == nil {
		return
	}
	slog.Info("Detected policy violation from audit logs",
		"policies", violation.Policies,
		"resource_name", violation.ResourceName,
		"namespace", violation.Namespace,
		"action", violation.Action,
		"audit_id", violation.AuditID,
		"metadata", violation.Metadata,
		"timestamp", violation.Timestamp)

	ts := violation.Timestamp
	if !violation.Timestamp.IsZero() {
		ts = violation.Timestamp
	}
	// Create a watched event from a policy violation event
	watchedEvent := &event.WatchedEvent{
		EventType:    event.EventTypeAdded,
		ResourceType: violation.ResourceType,
		Namespace:    violation.Namespace,
		Name:         fmt.Sprintf("policy-violation-%s-%s-%s", violation.ResourceType, violation.ResourceName, violation.AuditID),
		UID:          violation.AuditID,
		Timestamp:    ts.Unix(),
		EventTime:    ts.UTC().Format(time.RFC3339),
		Data: map[string]interface{}{
			"reason":   violation.Action,
			"type":     "Warning",
			"message":  violation.Message,
			"policies": violation.Policies,
			"source": map[string]interface{}{
				"component": "audit-log-handler",
			},
			"involvedObject": map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       violation.ResourceType,
				"name":       violation.ResourceName,
				"namespace":  violation.Namespace,
				"uid":        violation.AuditID,
			},
			"firstTimestamp": violation.Timestamp.Format(time.RFC3339),
			"lastTimestamp":  violation.Timestamp.Format(time.RFC3339),
			"count":          1,
			"metadata":       violation.Metadata,
		},
		Metadata: map[string]interface{}{
			"audit_id":      violation.AuditID,
			"metadata":      violation.Metadata,
			"policies":      violation.Policies,
			"resource_name": violation.ResourceName,
			"namespace":     violation.Namespace,
			"action":        violation.Action,
			"message":       violation.Message,
			"timestamp":     violation.Timestamp.Format(time.RFC3339),
			"event_time":    violation.Timestamp.Format(time.RFC3339),
		},
	}

	// Send the synthetic event to the event channel
	select {
	case h.eventChannel <- watchedEvent:
		slog.Debug("Sent watched event",
			"policies", violation.Policies,
			"resource_name", violation.ResourceName)
	default:
		slog.Warn("Event channel full, dropping watched event",
			"policies", violation.Policies,
			"resource_name", violation.ResourceName)
	}
}

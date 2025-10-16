package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
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
	logrus.WithField("audit_log_path", h.auditLogPath).Info("Starting audit log monitoring")

	// Check if audit log file exists
	if _, err := os.Stat(h.auditLogPath); os.IsNotExist(err) {
		logrus.WithField("audit_log_path", h.auditLogPath).Warn("Audit log file does not exist, audit log monitoring disabled")
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
		logrus.WithError(err).WithField("audit_log_path", h.auditLogPath).Error("Failed to open audit log file")
		return
	}
	defer file.Close()

	// Get file info to track position
	fileInfo, err := file.Stat()
	if err != nil {
		logrus.WithError(err).Error("Failed to get file info")
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
			logrus.WithError(err).WithFields(logrus.Fields{
				"line_number":    lineNumber,
				"audit_log_path": h.auditLogPath,
			}).Debug("Failed to parse audit log line")
			continue
		}

		// Check if this is a policy violation
		if violation := h.analyzeAuditEvent(auditEvent); violation != nil {
			h.generateSyntheticEvent(violation)
		}
	}

	if err := scanner.Err(); err != nil {
		logrus.WithError(err).WithField("audit_log_path", h.auditLogPath).Error("Error reading audit log file")
	}

	logrus.WithFields(logrus.Fields{
		"file_size":       fileInfo.Size(),
		"lines_processed": lineNumber,
	}).Debug("Processed audit log entries")
}

// analyzeAuditEvent analyzes an audit event to detect policy violations
func (h *AuditLogHandler) analyzeAuditEvent(auditEvent AuditEvent) *PolicyViolationEvent {
	// Only process deployment creation requests
	if auditEvent.ObjectRef.Resource != "deployments" || auditEvent.Verb != "create" {
		return nil
	}

	// Check if the request was blocked (HTTP 4xx or 5xx)
	if auditEvent.ResponseStatus.Code >= 400 {
		// This is a blocked request - extract policy violation information
		policyName := h.extractPolicyName(auditEvent.ResponseStatus.Message)

		logrus.WithFields(logrus.Fields{
			"audit_id":      auditEvent.AuditID,
			"policy_name":   policyName,
			"resource_name": auditEvent.ObjectRef.Name,
			"namespace":     auditEvent.ObjectRef.Namespace,
			"response_code": auditEvent.ResponseStatus.Code,
			"message":       auditEvent.ResponseStatus.Message,
			"level":         auditEvent.Level,
			"stage":         auditEvent.Stage,
		}).Info("Detected policy violation from audit logs")

		return &PolicyViolationEvent{
			Timestamp:    auditEvent.RequestReceivedTimestamp,
			PolicyName:   policyName,
			ResourceType: "Deployment",
			ResourceName: auditEvent.ObjectRef.Name,
			Namespace:    auditEvent.ObjectRef.Namespace,
			User:         auditEvent.User.Username,
			Action:       "blocked",
			Message:      auditEvent.ResponseStatus.Message,
			AuditID:      auditEvent.AuditID,
		}
	}

	return nil
}

// PolicyViolationEvent represents a detected policy violation from audit logs
type PolicyViolationEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	PolicyName   string    `json:"policy_name"`
	ResourceType string    `json:"resource_type"`
	ResourceName string    `json:"resource_name"`
	Namespace    string    `json:"namespace"`
	User         string    `json:"user"`
	Action       string    `json:"action"` // "blocked" or "allowed"
	Message      string    `json:"message"`
	AuditID      string    `json:"audit_id"`
}

// extractPolicyName extracts the policy name from the response message
func (h *AuditLogHandler) extractPolicyName(message string) string {
	// Try to extract policy name from message like "ValidatingAdmissionPolicy 'policy-name' denied request"
	if strings.Contains(message, "ValidatingAdmissionPolicy") {
		start := strings.Index(message, "'")
		if start != -1 {
			end := strings.Index(message[start+1:], "'")
			if end != -1 {
				return message[start+1 : start+1+end]
			}
		}
	}
	return "unknown-policy"
}

// generateSyntheticEvent creates a synthetic PolicyViolation event from audit log data
func (h *AuditLogHandler) generateSyntheticEvent(violation *PolicyViolationEvent) {
	logrus.WithFields(logrus.Fields{
		"policy_name":   violation.PolicyName,
		"resource_name": violation.ResourceName,
		"namespace":     violation.Namespace,
		"action":        violation.Action,
		"audit_id":      violation.AuditID,
	}).Info("Detected policy violation from audit logs")

	// Create a synthetic event that mimics a PolicyViolation event
	syntheticEvent := &event.WatchedEvent{
		EventType:    event.EventTypeAdded,
		ResourceType: "events",
		Namespace:    violation.Namespace,
		Name:         fmt.Sprintf("policy-violation-%s-%d", violation.ResourceName, time.Now().UnixNano()),
		UID:          violation.AuditID,
		Timestamp:    violation.Timestamp.Unix(),
		Data: map[string]interface{}{
			"reason":  "PolicyViolation",
			"type":    "Warning",
			"message": fmt.Sprintf("policy %s fail: %s", violation.PolicyName, violation.Message),
			"source": map[string]interface{}{
				"component": "audit-log-handler",
			},
			"involvedObject": map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"name":       violation.ResourceName,
				"namespace":  violation.Namespace,
				"uid":        violation.AuditID,
			},
			"firstTimestamp": violation.Timestamp.Format(time.RFC3339),
			"lastTimestamp":  violation.Timestamp.Format(time.RFC3339),
			"count":          1,
		},
	}

	// Send the synthetic event to the event channel
	select {
	case h.eventChannel <- syntheticEvent:
		logrus.WithFields(logrus.Fields{
			"policy_name":   violation.PolicyName,
			"resource_name": violation.ResourceName,
		}).Debug("Sent synthetic PolicyViolation event")
	default:
		logrus.WithFields(logrus.Fields{
			"policy_name":   violation.PolicyName,
			"resource_name": violation.ResourceName,
		}).Warn("Event channel full, dropping synthetic PolicyViolation event")
	}
}

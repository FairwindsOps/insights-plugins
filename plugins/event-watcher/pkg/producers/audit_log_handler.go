package producers

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"log/slog"

	"k8s.io/client-go/kubernetes"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/utils"

	"github.com/allegro/bigcache/v3"
)

var alreadyProcessedAuditIDs *bigcache.BigCache

func init() {
	var err error
	config := bigcache.DefaultConfig(60 * time.Minute)
	config.HardMaxCacheSize = 256 // 512MB
	alreadyProcessedAuditIDs, err = bigcache.New(context.Background(), config)
	if err != nil {
		slog.Error("Failed to create bigcache", "error", err)
	}
	slog.Info("Bigcache created", "size", alreadyProcessedAuditIDs.Len(), "hard_max_cache_size", config.HardMaxCacheSize)
}

// AuditLogHandler handles audit log monitoring and policy violation detection
type AuditLogHandler struct {
	insightsConfig models.InsightsConfig
	kubeClient     kubernetes.Interface
	auditLogPath   string
	eventChannel   chan *models.WatchedEvent
	stopCh         chan struct{}
}

// NewAuditLogHandler creates a new audit log handler
func NewAuditLogHandler(config models.InsightsConfig, kubeClient kubernetes.Interface, auditLogPath string, eventChannel chan *models.WatchedEvent) *AuditLogHandler {
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
	if h != nil && h.stopCh != nil {
		close(h.stopCh)
	}
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
		var auditEvent models.AuditEvent
		if err := json.Unmarshal([]byte(line), &auditEvent); err != nil {
			slog.Info("Failed to parse audit log line",
				"error", err,
				"line_number", lineNumber,
				"audit_log_path", h.auditLogPath)
			continue
		}

		if value, err := alreadyProcessedAuditIDs.Get(auditEvent.AuditID); err == nil && value != nil {
			slog.Debug("Audit ID already processed, skipping", "audit_id", auditEvent.AuditID)
			continue
		}

		if utils.IsKyvernoPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) ||
			utils.IsValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) ||
			utils.IsValidatingAdmissionPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {

			err = alreadyProcessedAuditIDs.Set(auditEvent.AuditID, []byte("true"))
			if err != nil {
				slog.Warn("Failed to set audit ID in bigcache", "error", err, "audit_id", auditEvent.AuditID)
			}
			policyViolationEvent := utils.CreateBlockedPolicyViolationEvent(auditEvent)
			slog.Debug("Checking if policy violation event is created", "policy_violation_event", policyViolationEvent)
			if policyViolationEvent != nil {
				slog.Info("Creating watched event from policy violation event", "policy_violation_event", policyViolationEvent)
				utils.CreateBlockedWatchedEventFromPolicyViolationEvent(policyViolationEvent, h.eventChannel)
			}
		} else if utils.IsValidatingAdmissionPolicyViolationAuditOnlyAllowEvent(auditEvent.Annotations) {
			err = alreadyProcessedAuditIDs.Set(auditEvent.AuditID, []byte("true"))
			if err != nil {
				slog.Warn("Failed to set audit ID in bigcache", "error", err, "audit_id", auditEvent.AuditID)
			}
			auditOnlyAllowEvent := utils.CreateValidatingAdmissionPolicyViolationAuditOnlyAllowEvent(auditEvent)
			slog.Debug("Checking if validating admission policy violation audit only allow event is created", "validating_admission_policy_violation_audit_only_allow_event", auditOnlyAllowEvent)
			if auditOnlyAllowEvent != nil {
				slog.Info("Creating watched event from validating admission policy violation audit only allow event", "validating_admission_policy_violation_audit_only_allow_event", auditOnlyAllowEvent)
				utils.CreateAuditOnlyAllowWatchedEventFromValidatingAdmissionPolicyViolation(auditOnlyAllowEvent, h.eventChannel)
			}
		}

	}

	if err := scanner.Err(); err != nil {
		slog.Error("Error reading audit log file", "error", err, "audit_log_path", h.auditLogPath)
	}

	slog.Debug("Processed audit log entries",
		"file_size", fileInfo.Size(),
		"lines_processed", lineNumber)
}

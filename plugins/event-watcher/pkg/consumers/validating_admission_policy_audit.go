package consumers

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/utils"
	"golang.org/x/time/rate"
)

type AuditOnlyAllowedValidatingAdmissionPolicyHandler struct {
	insightsConfig models.InsightsConfig
	client         *http.Client
	rateLimiter    *rate.Limiter
}

func NewAuditOnlyAllowedValidatingAdmissionPolicyHandler(insightsConfig models.InsightsConfig, httpTimeoutSeconds, rateLimitPerMinute int) *AuditOnlyAllowedValidatingAdmissionPolicyHandler {
	return &AuditOnlyAllowedValidatingAdmissionPolicyHandler{
		insightsConfig: insightsConfig,
		client: &http.Client{
			Timeout: time.Duration(httpTimeoutSeconds) * time.Second,
		},
		rateLimiter: rate.NewLimiter(rate.Limit(rateLimitPerMinute)/60.0, 1),
	}
}

func (h *AuditOnlyAllowedValidatingAdmissionPolicyHandler) Handle(watchedEvent *models.WatchedEvent) error {
	logFields := []interface{}{
		"event_type", watchedEvent.EventType,
		"resource_type", watchedEvent.ResourceType,
		"namespace", watchedEvent.Namespace,
		"name", watchedEvent.Name,
	}

	slog.Info("Processing AuditOnlyAllowedValidatingAdmissionPolicy event", logFields...)

	// Add Kubernetes eventTime to log if available
	if watchedEvent.EventTime != "" {
		logFields = append(logFields, "event_time", watchedEvent.EventTime)
	}

	slog.Info("Processing AuditOnlyAllowedValidatingAdmissionPolicy event", logFields...)

	auditOnlyAllowedValidatingEvent, err := h.extractAuditOnlyAllowedValidatingAdmissionPolicy(watchedEvent)
	if err != nil {
		errorFields := append(logFields, "error", err)
		slog.Warn("Failed to extract audit only allowed validating admission policy", errorFields...)
		return fmt.Errorf("failed to extract audit only allowed validating admission policy: %w", err)
	}
	slog.Info("Sending audit only, allowed validating admission policy event log to Insights",
		"policies", auditOnlyAllowedValidatingEvent.Policies,
		"success", auditOnlyAllowedValidatingEvent.Success,
		"blocked", auditOnlyAllowedValidatingEvent.Blocked,
		"namespace", auditOnlyAllowedValidatingEvent.Namespace)

	return utils.SendToInsights(h.insightsConfig, h.client, h.rateLimiter, auditOnlyAllowedValidatingEvent)
}

func (h *AuditOnlyAllowedValidatingAdmissionPolicyHandler) extractAuditOnlyAllowedValidatingAdmissionPolicy(watchedEvent *models.WatchedEvent) (*models.PolicyViolationEvent, error) {
	if watchedEvent == nil {
		return nil, fmt.Errorf("watchedEvent is nil")
	}
	if watchedEvent.Data == nil {
		return nil, fmt.Errorf("event data is nil")
	}
	message, ok := watchedEvent.Data["message"].(string)
	if !ok || message == "" {
		return nil, fmt.Errorf("no message field in event or message is empty")
	}

	annotations, ok := watchedEvent.Data["annotations"].(map[string]string)
	if !ok {
		return nil, fmt.Errorf("no annotations field in event or annotations is not a map")
	}

	policies := utils.ExtractAuditOnlyAllowedValidatingAdmissionPoliciesFromMessage(annotations)
	return &models.PolicyViolationEvent{
		EventReport: models.EventReport{
			EventType:    string(watchedEvent.EventType),
			ResourceType: watchedEvent.ResourceType,
			Namespace:    watchedEvent.Namespace,
			Name:         watchedEvent.Name,
			UID:          watchedEvent.UID,
			Timestamp:    watchedEvent.Timestamp,
			Data:         watchedEvent.Data,
			Metadata:     watchedEvent.Metadata,
		},
		Policies:  policies,
		Message:   message,
		Success:   watchedEvent.Success,
		Blocked:   watchedEvent.Blocked,
		EventTime: watchedEvent.EventTime,
	}, nil
}

package consumers

import (
	"fmt"
	"net/http"
	"time"

	"log/slog"

	"golang.org/x/time/rate"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/utils"
)

type ValidatingAdmissionPolicyViolationHandler struct {
	insightsConfig models.InsightsConfig
	client         *http.Client
	rateLimiter    *rate.Limiter
}

func NewValidatingAdmissionPolicyViolationHandler(insightsConfig models.InsightsConfig, httpTimeoutSeconds, rateLimitPerMinute int) *ValidatingAdmissionPolicyViolationHandler {
	return &ValidatingAdmissionPolicyViolationHandler{
		insightsConfig: insightsConfig,
		client: &http.Client{
			Timeout: time.Duration(httpTimeoutSeconds) * time.Second,
		},
		rateLimiter: rate.NewLimiter(rate.Limit(rateLimitPerMinute)/60.0, 1),
	}
}

func (h *ValidatingAdmissionPolicyViolationHandler) Handle(watchedEvent *models.WatchedEvent) error {
	logFields := []any{
		"event_type", watchedEvent.EventType,
		"kind", watchedEvent.Kind,
		"namespace", watchedEvent.Namespace,
		"name", watchedEvent.Name,
	}

	slog.Info("Processing ValidatingAdmissionPolicyViolation event", logFields...)
	// Add Kubernetes eventTime to log if available
	if watchedEvent.EventTime != "" {
		logFields = append(logFields, "event_time", watchedEvent.EventTime)
	}

	slog.Info("Processing ValidatingAdmissionPolicyViolation event", logFields...)

	validatingEvent, err := h.extractValidatingAdmissionPolicyViolation(watchedEvent)
	if err != nil {
		errorFields := append(logFields, "error", err)
		slog.Warn("Failed to extract validating admission policy violation", errorFields...)
		return fmt.Errorf("failed to extract validating admission policy violation: %w", err)
	}
	slog.Info("Sending validating admission policy violation to Insights",
		"policies", validatingEvent.Policies,
		"success", validatingEvent.Success,
		"blocked", validatingEvent.Blocked,
		"namespace", validatingEvent.Namespace)

	return utils.SendToInsights(h.insightsConfig, h.client, h.rateLimiter, validatingEvent)
}

func (h *ValidatingAdmissionPolicyViolationHandler) extractValidatingAdmissionPolicyViolation(watchedEvent *models.WatchedEvent) (*models.PolicyViolationEvent, error) {
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

	policies := utils.ExtractValidatingAdmissionPoliciesFromMessage(message)
	return &models.PolicyViolationEvent{
		EventReport: models.EventReport{
			EventType: string(watchedEvent.EventType),
			Namespace: watchedEvent.Namespace,
			Name:      watchedEvent.Name,
			UID:       watchedEvent.UID,
			Timestamp: watchedEvent.Timestamp,
			Data:      watchedEvent.Data,
			Metadata:  watchedEvent.Metadata,
		},
		Policies:  policies,
		Message:   message,
		Blocked:   watchedEvent.Blocked,
		Success:   watchedEvent.Success,
		EventTime: watchedEvent.EventTime,
	}, nil
}

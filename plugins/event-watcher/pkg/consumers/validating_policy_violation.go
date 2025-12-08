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

type ValidatingPolicyViolationHandler struct {
	insightsConfig models.InsightsConfig
	client         *http.Client
	rateLimiter    *rate.Limiter
}

func NewValidatingPolicyViolationHandler(insightsConfig models.InsightsConfig, httpTimeoutSeconds, rateLimitPerMinute int) *ValidatingPolicyViolationHandler {
	return &ValidatingPolicyViolationHandler{
		insightsConfig: insightsConfig,
		client: &http.Client{
			Timeout: time.Duration(httpTimeoutSeconds) * time.Second,
		},
		rateLimiter: rate.NewLimiter(rate.Limit(rateLimitPerMinute)/60.0, 1),
	}
}

func (h *ValidatingPolicyViolationHandler) Handle(watchedEvent *models.WatchedEvent) error {
	if watchedEvent == nil {
		return fmt.Errorf("watchedEvent is nil")
	}
	logFields := []any{
		"event_type", watchedEvent.EventType,
		"kind", watchedEvent.Kind,
		"namespace", watchedEvent.Namespace,
		"name", watchedEvent.Name,
	}

	slog.Debug("Processing PolicyViolation event", logFields...)
	// Add Kubernetes eventTime to log if available
	if watchedEvent.EventTime != "" {
		logFields = append(logFields, "event_time", watchedEvent.EventTime)
	}

	slog.Debug("Processing PolicyViolation event", logFields...)

	// Add detailed logging to see what data we're processing
	if watchedEvent.Data != nil {
		debugFields := append(logFields, "event_data", watchedEvent.Data)
		slog.Debug("Event data received", debugFields...)

		// Log the message if it exists
		if message, ok := watchedEvent.Data["message"].(string); ok {
			messageFields := append(logFields, "message", message)
			slog.Debug("Event message", messageFields...)
		}

		// Log the reason if it exists
		if reason, ok := watchedEvent.Data["reason"].(string); ok {
			reasonFields := append(logFields, "reason", reason)
			slog.Debug("Event reason", reasonFields...)
		}
	}

	validatingEvent, err := h.extractValidatingPolicyViolation(watchedEvent)
	if err != nil {
		errorFields := append(logFields, "error", err)
		slog.Warn("Failed to extract validating policy violation", errorFields...)
		return fmt.Errorf("failed to extract validating policy violation: %w", err)
	}
	slog.Info("Sending validating policy violation to Insights",
		"policies", validatingEvent.Policies,
		"success", validatingEvent.Success,
		"blocked", validatingEvent.Blocked,
		"namespace", validatingEvent.Namespace)

	return utils.SendToInsights(h.insightsConfig, h.client, h.rateLimiter, validatingEvent)
}

func (h *ValidatingPolicyViolationHandler) extractValidatingPolicyViolation(watchedEvent *models.WatchedEvent) (*models.PolicyViolationEvent, error) {
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

	policies := utils.ExtractValidatingPoliciesFromMessage(message)
	return &models.PolicyViolationEvent{
		EventReport: models.EventReport{
			EventType:    string(watchedEvent.EventType),
			ResourceType: watchedEvent.Kind,
			Namespace:    watchedEvent.Namespace,
			Name:         watchedEvent.Name,
			UID:          watchedEvent.UID,
			Timestamp:    watchedEvent.Timestamp,
			Data:         watchedEvent.Data,
			Metadata:     watchedEvent.Metadata,
		},
		Policies:  policies,
		Message:   message,
		Blocked:   watchedEvent.Blocked,
		Success:   watchedEvent.Success,
		EventTime: watchedEvent.EventTime,
	}, nil
}

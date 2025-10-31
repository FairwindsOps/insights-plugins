package consumers

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"log/slog"

	"golang.org/x/time/rate"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/utils"
)

type PolicyViolationHandler struct {
	insightsConfig models.InsightsConfig
	client         *http.Client
	rateLimiter    *rate.Limiter
	mu             sync.Mutex
}

func NewPolicyViolationHandler(config models.InsightsConfig, httpTimeoutSeconds, rateLimitPerMinute int) *PolicyViolationHandler {
	// Create rate limiter: rateLimitPerMinute calls per minute
	limiter := rate.NewLimiter(rate.Limit(rateLimitPerMinute)/60.0, 1)

	return &PolicyViolationHandler{
		insightsConfig: config,
		client: &http.Client{
			Timeout: time.Duration(httpTimeoutSeconds) * time.Second,
		},
		rateLimiter: limiter,
	}
}

func (h *PolicyViolationHandler) Handle(watchedEvent *models.WatchedEvent) error {
	logFields := []interface{}{
		"event_type", watchedEvent.EventType,
		"resource_type", watchedEvent.ResourceType,
		"namespace", watchedEvent.Namespace,
		"name", watchedEvent.Name,
	}

	slog.Info("Processing PolicyViolation event", logFields...)
	// Add Kubernetes eventTime to log if available
	if watchedEvent.EventTime != "" {
		logFields = append(logFields, "event_time", watchedEvent.EventTime)
	}

	slog.Info("Processing PolicyViolation event", logFields...)

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

	violationEvent, err := h.extractPolicyViolation(watchedEvent)
	if err != nil {
		errorFields := append(logFields, "error", err)
		slog.Warn("Failed to extract policy violation", errorFields...)
		return fmt.Errorf("failed to extract policy violation: %w", err)
	}

	// Only send blocked policy violations to Insights (any type that blocks resource installation)
	if !violationEvent.Blocked {
		slog.Info("Policy violation is not blocked, skipping (only blocked policy violations are sent to Insights)",
			"policies", violationEvent.Policies,
			"success", violationEvent.Success,
			"blocked", violationEvent.Blocked,
			"namespace", violationEvent.Namespace,
			"resource", violationEvent.Name)
		return nil
	}

	slog.Info("Sending blocked policy violation to Insights",
		"policies", violationEvent.Policies,
		"success", violationEvent.Success,
		"blocked", violationEvent.Blocked,
		"namespace", violationEvent.Namespace,
		"resource", violationEvent.Name,
		"blocked", violationEvent.Blocked)

	return utils.SendToInsights(h.insightsConfig, h.client, h.rateLimiter, violationEvent)
}

func (h *PolicyViolationHandler) extractPolicyViolation(watchedEvent *models.WatchedEvent) (*models.PolicyViolationEvent, error) {
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

	policies := utils.ExtractPoliciesFromMessage(message)

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
		Blocked:   watchedEvent.Blocked,
		Success:   watchedEvent.Success,
		EventTime: watchedEvent.EventTime,
	}, nil
}

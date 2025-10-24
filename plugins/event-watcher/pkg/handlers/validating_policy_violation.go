package handlers

import (
	"fmt"
	"net/http"
	"time"

	"log/slog"

	"golang.org/x/time/rate"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
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

/*
We should look for the response
admission webhook "vpol.validate.kyverno.svc-fail" denied the request: Policy check-labels failed: label 'environment' is required: failed to create resource: admission webhook "vpol.validate.kyverno.svc-fail" denied the request: Policy check-labels failed: label 'environment' is required
*/

func (h *ValidatingPolicyViolationHandler) Handle(watchedEvent *event.WatchedEvent) error {
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

	validatingEvent, err := h.extractValidatingPolicyViolation(watchedEvent)
	if err != nil {
		errorFields := append(logFields, "error", err)
		slog.Warn("Failed to extract validating policy violation", errorFields...)
		return fmt.Errorf("failed to extract validating policy violation: %w", err)
	}
	slog.Info("Sending validating policy violation to Insights",
		"policies", validatingEvent.Policies,
		"result", validatingEvent.PolicyResult,
		"namespace", validatingEvent.Namespace)

	return SendToInsights(h.insightsConfig, h.client, h.rateLimiter, validatingEvent)
}

func (h *ValidatingPolicyViolationHandler) extractValidatingPolicyViolation(watchedEvent *event.WatchedEvent) (*models.PolicyViolationEvent, error) {
	return nil, nil
}

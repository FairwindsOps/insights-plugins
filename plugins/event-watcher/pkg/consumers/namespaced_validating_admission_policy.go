package consumers

import (
	"fmt"
	"net/http"
	"time"

	"log/slog"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/utils"
	"golang.org/x/time/rate"
)

type NamespacedValidatingAdmissionPolicyViolationHandler struct {
	insightsConfig models.InsightsConfig
	client         *http.Client
	rateLimiter    *rate.Limiter
}

func NewNamespacedValidatingPolicyViolationHandler(insightsConfig models.InsightsConfig, httpTimeoutSeconds, rateLimitPerMinute int) *NamespacedValidatingAdmissionPolicyViolationHandler {
	return &NamespacedValidatingAdmissionPolicyViolationHandler{
		insightsConfig: insightsConfig,
		client: &http.Client{
			Timeout: time.Duration(httpTimeoutSeconds) * time.Second,
		},
		rateLimiter: rate.NewLimiter(rate.Limit(rateLimitPerMinute)/60.0, 1),
	}
}

func (h *NamespacedValidatingAdmissionPolicyViolationHandler) Handle(watchedEvent *models.WatchedEvent) error {
	if watchedEvent == nil {
		return fmt.Errorf("watchedEvent is nil")
	}
	logFields := []any{
		"event_type", watchedEvent.EventType,
		"kind", watchedEvent.Kind,
		"namespace", watchedEvent.Namespace,
		"name", watchedEvent.Name,
	}

	slog.Info("Processing NamespacedValidatingAdmissionPolicyViolation event", logFields...)
	policyViolationEvent, err := h.extractNamespacedValidatingAdmissionPolicyViolation(watchedEvent)
	if err != nil {
		return fmt.Errorf("failed to extract namespaced validating admission policy violation: %w", err)
	}
	return h.sendToInsights(policyViolationEvent)
}

func (h *NamespacedValidatingAdmissionPolicyViolationHandler) extractNamespacedValidatingAdmissionPolicyViolation(watchedEvent *models.WatchedEvent) (*models.PolicyViolationEvent, error) {
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

	policies := utils.ExtractNamespacedValidatingPoliciesFromMessage(message)
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

func (h *NamespacedValidatingAdmissionPolicyViolationHandler) sendToInsights(policyViolationEvent *models.PolicyViolationEvent) error {
	return utils.SendToInsights(h.insightsConfig, h.client, h.rateLimiter, policyViolationEvent)
}

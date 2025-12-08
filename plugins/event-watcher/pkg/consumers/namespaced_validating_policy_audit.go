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

type NamespacedValidatingPolicyAuditHandler struct {
	insightsConfig models.InsightsConfig
	client         *http.Client
	rateLimiter    *rate.Limiter
}

func NewNamespacedValidatingPolicyAuditHandler(config models.InsightsConfig, httpTimeoutSeconds, rateLimitPerMinute int) *NamespacedValidatingPolicyAuditHandler {
	limiter := rate.NewLimiter(rate.Limit(rateLimitPerMinute)/60.0, 1)
	return &NamespacedValidatingPolicyAuditHandler{
		insightsConfig: config,
		client:         &http.Client{Timeout: time.Duration(httpTimeoutSeconds) * time.Second},
		rateLimiter:    limiter,
	}
}

func (h *NamespacedValidatingPolicyAuditHandler) Handle(watchedEvent *models.WatchedEvent) error {
	if watchedEvent == nil {
		return fmt.Errorf("watchedEvent is nil")
	}
	logFields := []any{
		"event_type", watchedEvent.EventType,
		"kind", watchedEvent.Kind,
		"namespace", watchedEvent.Namespace,
		"name", watchedEvent.Name,
	}

	slog.Info("Processing NamespacedValidatingPolicyAudit event", logFields...)
	if watchedEvent.Metadata == nil || watchedEvent.Metadata["message"] == nil {
		return fmt.Errorf("event metadata is nil or message is nil in event %+v", watchedEvent)
	}
	message, ok := watchedEvent.Metadata["message"].(string)
	if !ok {
		return fmt.Errorf("message is not a string in event %+v", watchedEvent)
	}
	policyName, ok := watchedEvent.Metadata["policyName"].(string)
	if !ok {
		return fmt.Errorf("policyName is not a string in event %+v", watchedEvent)
	}
	policies := utils.ExtractAuditOnlyNamespacedValidatingPoliciesFromMessage(policyName, message)
	slog.Info("Sending namespaced validating policy audit to Insights", "policies", policies, "message", message, "blocked", watchedEvent.Blocked)
	err := utils.SendToInsights(h.insightsConfig, h.client, h.rateLimiter, &models.PolicyViolationEvent{
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
	})
	if err != nil {
		return fmt.Errorf("failed to send namespaced validating policy audit to Insights: %w", err)
	}
	return nil
}

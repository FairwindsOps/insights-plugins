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

type ClusterPolicyAuditHandler struct {
	insightsConfig models.InsightsConfig
	client         *http.Client
	rateLimiter    *rate.Limiter
}

func NewClusterPolicyAuditHandler(config models.InsightsConfig, httpTimeoutSeconds, rateLimitPerMinute int) *ClusterPolicyAuditHandler {
	limiter := rate.NewLimiter(rate.Limit(rateLimitPerMinute)/60.0, 1)
	return &ClusterPolicyAuditHandler{
		insightsConfig: config,
		client:         &http.Client{Timeout: time.Duration(httpTimeoutSeconds) * time.Second},
		rateLimiter:    limiter,
	}
}

/*

event := &models.WatchedEvent{
			EventVersion: EventVersion,
			Timestamp:    event.EventTime.Unix(),
			EventTime:    event.EventTime.UTC().Format(time.RFC3339),
			EventType:    models.EventTypeAdded,
			Kind:         event.Related.Kind,
			Namespace:    event.Related.Namespace,
			Name:         fmt.Sprintf("%s-%s-%s-%s", utils.AuditOnlyAllowedValidatingAdmissionPolicyPrefix, event.Related.Kind, event.Related.Name, event.ObjectMeta.UID),
			UID:          string(event.Related.UID),
			Data: map[string]interface{}{
				"event": event,
			},
			Metadata: map[string]interface{}{
				"annotations":       event.ObjectMeta.Annotations,
				"labels":            event.ObjectMeta.Labels,
				"creationTimestamp": event.ObjectMeta.CreationTimestamp.Format(time.RFC3339),
				"resourceVersion":   event.ObjectMeta.ResourceVersion,
				"uid":               event.ObjectMeta.UID,
				"reason":            event.Reason,
				"message":           event.Message,
				"involvedObject":    event.InvolvedObject,
				"related":           event.Related,
				"reportingInstance": event.ReportingInstance,
				"source":            event.Source,
				"type":              event.Type,
			},
			EventSource: "kubernetes_events",
			Success:     false,
			Blocked:     true,
		}

*/

func (h *ClusterPolicyAuditHandler) Handle(watchedEvent *models.WatchedEvent) error {
	logFields := []interface{}{
		"event_type", watchedEvent.EventType,
		"kind", watchedEvent.Kind,
		"namespace", watchedEvent.Namespace,
		"name", watchedEvent.Name,
	}

	slog.Info("Processing ClusterPolicyAudit event", logFields...)
	if watchedEvent.Data == nil || watchedEvent.Data["message"] == nil {
		return fmt.Errorf("event data is nil or message is nil in event %+v", watchedEvent)
	}
	message, ok := watchedEvent.Data["message"].(string)
	if !ok {
		return fmt.Errorf("message is not a string in event %+v", watchedEvent)
	}
	policies := utils.ExtractAuditOnlyClusterPoliciesFromMessage(message)
	slog.Info("Sending cluster policy audit to Insights", "policies", policies, "message", message)
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
		Blocked:   false,
		Success:   true,
		EventTime: watchedEvent.EventTime,
	})
	if err != nil {
		slog.Error("Failed to send cluster policy audit to Insights", "error", err)
		return fmt.Errorf("failed to send cluster policy audit to Insights: %w", err)
	}
	return nil
}

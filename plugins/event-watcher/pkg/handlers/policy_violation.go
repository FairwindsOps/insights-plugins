package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"log/slog"

	"golang.org/x/time/rate"

	version "github.com/fairwindsops/insights-plugins/plugins/event-watcher"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
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

func (h *PolicyViolationHandler) Handle(watchedEvent *event.WatchedEvent) error {
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
			"result", violationEvent.PolicyResult,
			"namespace", violationEvent.Namespace,
			"resource", violationEvent.Name)
		return nil
	}

	slog.Info("Sending blocked policy violation to Insights",
		"policies", violationEvent.Policies,
		"result", violationEvent.PolicyResult,
		"namespace", violationEvent.Namespace,
		"resource", violationEvent.Name,
		"blocked", violationEvent.Blocked)

	return h.sendToInsights(violationEvent)
}

func (h *PolicyViolationHandler) extractPolicyViolation(watchedEvent *event.WatchedEvent) (*models.PolicyViolationEvent, error) {
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

	policies := ExtractPoliciesFromMessage(message)
	blocked := false
	policyResult := ""
	if watchedEvent.Metadata != nil && watchedEvent.Metadata["policyResult"] != nil {
		policyResult, ok := watchedEvent.Metadata["policyResult"].(string)
		if !ok {
			slog.Warn("No policy result found in metadata, blocked is set to true", "metadata", watchedEvent.Metadata)
		} else {
			blocked = policyResult == "fail"
		}
	} else {
		slog.Warn("No policy result found in metadata, blocked is set to true", "metadata", watchedEvent.Metadata)
		blocked = true
		policyResult = "fail"
	}
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
		Policies:     policies,
		PolicyResult: policyResult,
		Message:      message,
		Blocked:      blocked,
		EventTime:    watchedEvent.EventTime,
	}, nil
}

// sendToInsights sends the policy violation to Insights API
func (h *PolicyViolationHandler) sendToInsights(violationEvent *models.PolicyViolationEvent) error {
	// Apply rate limiting
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := h.rateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit exceeded: %w", err)
	}

	// Convert to JSON
	jsonData, err := json.Marshal(violationEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal violation event: %w", err)
	}

	url := fmt.Sprintf("%s/v0/organizations/%s/clusters/%s/data/watcher/policy-violations",
		h.insightsConfig.Hostname,
		h.insightsConfig.Organization,
		h.insightsConfig.Cluster)

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.insightsConfig.Token)

	watcherVersion := version.Version
	req.Header.Set("X-Fairwinds-Watcher-Version", watcherVersion)

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("insights API returned status %d", resp.StatusCode)
	}

	slog.Info("Successfully sent blocked policy violation to Insights API",
		"policies", violationEvent.Policies,
		"result", violationEvent.PolicyResult,
		"blocked", violationEvent.Blocked,
		"namespace", violationEvent.Namespace,
		"resource", violationEvent.Name,
		"event_time", violationEvent.EventTime,
		"timestamp", violationEvent.Timestamp)

	return nil
}

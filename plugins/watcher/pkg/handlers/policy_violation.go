package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"log/slog"

	"golang.org/x/time/rate"

	version "github.com/fairwindsops/insights-plugins/plugins/watcher"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
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
		slog.Debug("Policy violation is not blocked, skipping (only blocked policy violations are sent to Insights)",
			"policy_name", violationEvent.PolicyName,
			"result", violationEvent.PolicyResult,
			"namespace", violationEvent.Namespace,
			"resource", violationEvent.Name)
		return nil
	}

	slog.Info("Sending blocked policy violation to Insights",
		"policy_name", violationEvent.PolicyName,
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

	policyName, policyResult, blocked, err := h.parsePolicyMessage(message)
	if err != nil {
		return nil, fmt.Errorf("failed to parse policy message: %w", err)
	}

	involvedObject, ok := watchedEvent.Data["involvedObject"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no involvedObject field in event or invalid format")
	}

	violationEvent := &models.PolicyViolationEvent{
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
		PolicyName:   policyName,
		PolicyResult: policyResult,
		Message:      message,
		Blocked:      blocked,
	}

	// Use extracted Kubernetes eventTime
	violationEvent.EventTime = watchedEvent.EventTime

	if kind, ok := involvedObject["kind"].(string); ok {
		violationEvent.ResourceType = kind
	}
	if name, ok := involvedObject["name"].(string); ok {
		violationEvent.Name = name
	}
	if namespace, ok := involvedObject["namespace"].(string); ok {
		violationEvent.Namespace = namespace
	}

	return violationEvent, nil
}

func (h *PolicyViolationHandler) parsePolicyMessage(message string) (policyName, policyResult string, blocked bool, err error) {
	if message == "" {
		return "", "", false, fmt.Errorf("empty message")
	}

	slog.Debug("Parsing policy message", "message", message)

	blocked = strings.Contains(message, " (blocked)") || strings.HasSuffix(message, "(blocked)")

	// Try to parse the new Kyverno format first: "policy namespace/policy-name result: description"
	if strings.HasPrefix(message, "policy ") {
		parts := strings.Fields(message)
		if len(parts) >= 3 {
			// Format: "policy disallow-host-path/disallow-host-path fail: description"
			policyName = parts[1]   // "disallow-host-path/disallow-host-path"
			policyResult = parts[2] // "fail"
			// Remove trailing colon if present
			policyResult = strings.TrimSuffix(policyResult, ":")

			// Validate policy result
			if policyResult != "fail" && policyResult != "warn" && policyResult != "pass" && policyResult != "error" {
				policyResult = "unknown"
			}

			// For Kyverno format, check if the message contains "(blocked)" to determine if it's blocked
			blocked = strings.Contains(message, "(blocked)")

			return policyName, policyResult, blocked, nil
		}
	}

	// Try to parse ValidatingAdmissionPolicy warning format: "Warning: Validation failed for ValidatingAdmissionPolicy 'policy-name' with binding 'binding-name': description"
	if strings.HasPrefix(message, "Warning: Validation failed for ValidatingAdmissionPolicy ") {
		// Extract policy name from the message
		// Format: "Warning: Validation failed for ValidatingAdmissionPolicy 'policy-name' with binding 'binding-name': description"
		start := strings.Index(message, "'")
		if start != -1 {
			end := strings.Index(message[start+1:], "'")
			if end != -1 {
				policyName = message[start+1 : start+1+end]
				policyResult = "fail" // ValidatingAdmissionPolicy warnings are always failures
				// For audit policies, it's not blocked (just logged)
				// For enforce policies, it would be blocked, but we can't determine this from the message alone
				// We'll assume it's blocked if it's not an audit policy
				blocked = !strings.Contains(policyName, "-insights-audit")
				return policyName, policyResult, blocked, nil
			}
		}
	}

	// Try to parse ValidatingAdmissionPolicy format: "Deployment default/nginx: [policy-name] result; description"
	// This format is used by ValidatingAdmissionPolicy resources
	start := strings.Index(message, "[")
	end := strings.Index(message, "]")
	if start != -1 && end != -1 && start < end && end > start {
		policyName = message[start+1 : end]

		// Validate policy name is not empty
		if policyName == "" {
			return "", "", false, fmt.Errorf("empty policy name in brackets")
		}

		// Look for result after the closing bracket
		afterBracket := message[end+1:]
		parts := strings.Fields(afterBracket)
		for i, part := range parts {
			// Remove semicolon if present
			cleanPart := strings.TrimSuffix(part, ";")
			if cleanPart == "fail" || cleanPart == "warn" || cleanPart == "pass" {
				policyResult = cleanPart
				break
			}
			// Handle "validation error" format: "validation error" means "fail"
			if cleanPart == "error" && i > 0 && parts[i-1] == "validation" {
				policyResult = "fail"
				break
			}
		}

		if policyResult == "" {
			policyResult = "unknown"
		}

		return policyName, policyResult, blocked, nil
	}

	return "", "", false, fmt.Errorf("could not parse policy message format: %s", message)
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
		"policy_name", violationEvent.PolicyName,
		"policy_result", violationEvent.PolicyResult,
		"blocked", violationEvent.Blocked,
		"namespace", violationEvent.Namespace,
		"resource", violationEvent.Name)

	return nil
}

package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	version "github.com/fairwindsops/insights-plugins/plugins/watcher"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
)

type PolicyViolationHandler struct {
	insightsConfig models.InsightsConfig
	client         *http.Client
}

func NewPolicyViolationHandler(config models.InsightsConfig) *PolicyViolationHandler {
	return &PolicyViolationHandler{
		insightsConfig: config,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (h *PolicyViolationHandler) Handle(watchedEvent *event.WatchedEvent) error {
	logrus.WithFields(logrus.Fields{
		"event_type":    watchedEvent.EventType,
		"resource_type": watchedEvent.ResourceType,
		"namespace":     watchedEvent.Namespace,
		"name":          watchedEvent.Name,
	}).Info("Processing PolicyViolation event")

	violationEvent, err := h.extractPolicyViolation(watchedEvent)
	if err != nil {
		return fmt.Errorf("failed to extract policy violation: %w", err)
	}

	// Only send blocked policy violations to Insights (any type that blocks resource installation)
	if !violationEvent.Blocked {
		logrus.WithFields(logrus.Fields{
			"policy_name": violationEvent.PolicyName,
			"result":      violationEvent.PolicyResult,
			"namespace":   violationEvent.Namespace,
			"resource":    violationEvent.Name,
		}).Debug("Policy violation is not blocked, skipping (only blocked policy violations are sent to Insights)")
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"policy_name": violationEvent.PolicyName,
		"result":      violationEvent.PolicyResult,
		"namespace":   violationEvent.Namespace,
		"resource":    violationEvent.Name,
		"blocked":     violationEvent.Blocked,
	}).Info("Sending blocked policy violation to Insights")

	return h.sendToInsights(violationEvent)
}

func (h *PolicyViolationHandler) extractPolicyViolation(watchedEvent *event.WatchedEvent) (*models.PolicyViolationEvent, error) {
	message, ok := watchedEvent.Data["message"].(string)
	if !ok {
		return nil, fmt.Errorf("no message field in event")
	}

	policyName, policyResult, blocked, err := h.parsePolicyMessage(message)
	if err != nil {
		return nil, fmt.Errorf("failed to parse policy message: %w", err)
	}

	involvedObject, ok := watchedEvent.Data["involvedObject"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no involvedObject field in event")
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
	blocked = strings.Contains(message, "(blocked)")

	// Try to parse the new Kyverno format first: "policy namespace/policy-name result: description"
	if strings.HasPrefix(message, "policy ") {
		parts := strings.Fields(message)
		if len(parts) >= 3 {
			// Format: "policy disallow-host-path/disallow-host-path fail: description"
			policyName = parts[1]   // "disallow-host-path/disallow-host-path"
			policyResult = parts[2] // "fail"
			// Remove trailing colon if present
			policyResult = strings.TrimSuffix(policyResult, ":")
			return policyName, policyResult, blocked, nil
		}
	}

	// Try to parse ValidatingAdmissionPolicy format: "Deployment default/nginx: [policy-name] result; description"
	// This format is used by ValidatingAdmissionPolicy resources
	start := strings.Index(message, "[")
	end := strings.Index(message, "]")
	if start != -1 && end != -1 && start < end {
		policyName = message[start+1 : end]

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
func (h *PolicyViolationHandler) sendToInsights(violationEvent *models.PolicyViolationEvent) error { // Convert to JSON
	jsonData, err := json.Marshal(violationEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal violation event: %w", err)
	}

	url := fmt.Sprintf("%s/v0/organizations/%s/clusters/%s/data/watcher/policy-violations",
		h.insightsConfig.Hostname,
		h.insightsConfig.Organization,
		h.insightsConfig.Cluster)

	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
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

	logrus.WithFields(logrus.Fields{
		"policy_name":   violationEvent.PolicyName,
		"policy_result": violationEvent.PolicyResult,
		"blocked":       violationEvent.Blocked,
		"namespace":     violationEvent.Namespace,
		"resource":      violationEvent.Name,
	}).Info("Successfully sent blocked policy violation to Insights API")

	return nil
}

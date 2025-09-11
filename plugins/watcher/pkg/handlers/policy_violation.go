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

	if !violationEvent.Blocked {
		logrus.Debug("Policy violation is not blocked, skipping")
		return nil
	}

	return h.sendToInsights(violationEvent)
}

func (h *PolicyViolationHandler) extractPolicyViolation(watchedEvent *event.WatchedEvent) (*models.PolicyViolationEvent, error) {
	message, ok := watchedEvent.Data["message"].(string)
	if !ok {
		return nil, fmt.Errorf("no message field in event")
	}

	// Parse the message to extract policy details
	// Example: "Pod default/nginx: [require-team-label] fail (blocked); validation error: The label 'team' is required for all Pods. rule require-team-label failed at path /metadata/labels/team/"
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

	start := strings.Index(message, "[")
	end := strings.Index(message, "]")
	if start == -1 || end == -1 || start >= end {
		return "", "", false, fmt.Errorf("could not find policy name in brackets")
	}
	policyName = message[start+1 : end]

	parts := strings.Fields(message)
	for i, part := range parts {
		if part == "fail" || part == "warn" || part == "pass" {
			policyResult = part
			break
		}
		if part == "error" && i > 0 && parts[i-1] == "validation" {
			policyResult = "fail"
			break
		}
	}

	if policyResult == "" {
		policyResult = "unknown"
	}

	return policyName, policyResult, blocked, nil
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
	}).Info("Successfully sent policy violation to Insights API")

	return nil
}

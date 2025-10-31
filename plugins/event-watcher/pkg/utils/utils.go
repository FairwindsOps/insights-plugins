package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	version "github.com/fairwindsops/insights-plugins/plugins/event-watcher"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
	"github.com/ghodss/yaml"
	"golang.org/x/time/rate"
)

const (
	KyvernoPolicyViolationPrefix             = "kyverno-policy-violation"
	ValidatingPolicyViolationPrefix          = "validating-policy-violation"
	ValidatingAdmissionPolicyViolationPrefix = "validating-admission-policy-violation"
)

func ExtractPoliciesFromMessage(message string) map[string]map[string]string {
	policies := map[string]map[string]string{}
	allPolicies := ""
	if strings.Contains(message, "admission webhook") && strings.Contains(message, "denied the request:") {
		expectedText := "due to the following policies"
		start := strings.Index(message, expectedText)
		if start != -1 {
			start = start + len(expectedText)
			allPolicies = message[start:]
		}
	}
	err := yaml.Unmarshal([]byte(allPolicies), &policies)
	if err != nil {
		slog.Error("Failed to unmarshal policies", "error", err)
		return map[string]map[string]string{}
	}
	return policies
}

func ExtractValidatingPoliciesFromMessage(message string) map[string]map[string]string {
	policyName := "unknown"
	if strings.Contains(message, "vpol") && strings.Contains(message, "kyverno") && strings.Contains(message, "denied the request:") {
		startIndex := strings.Index(message, "denied the request: Policy") + len("denied the request: Policy")
		endIndex := strings.Index(message, " failed:")
		if startIndex != -1 && endIndex != -1 {
			policyName = message[startIndex:endIndex]
			policyName = strings.TrimSpace(policyName)
		}
	}
	return map[string]map[string]string{
		policyName: {
			policyName: message,
		},
	}
}

func ExtractValidatingAdmissionPoliciesFromMessage(message string) map[string]map[string]string {
	// Parsing example: "deployments.apps \"nginx-deployment\" is forbidden: ValidatingAdmissionPolicy 'check-deployment-replicas' with binding 'check-deployment-replicas-binding' denied request: failed expression: object.spec.replicas >= 5"
	if strings.Contains(message, "ValidatingAdmissionPolicy") && strings.Contains(message, "denied request:") {
		policyName := "unknown"
		if strings.Contains(message, "ValidatingAdmissionPolicy") {
			policyName = message[strings.Index(message, "ValidatingAdmissionPolicy")+len("ValidatingAdmissionPolicy") : strings.Index(message, " with binding ")]
			// remove quotes
			policyName = strings.ReplaceAll(policyName, "'", "")
			policyName = strings.TrimSpace(policyName)
		}
		return map[string]map[string]string{
			policyName: {
				policyName: message,
			},
		}
	}
	return map[string]map[string]string{}
}

// sendToInsights sends the policy violation to Insights API
func SendToInsights(insightsConfig models.InsightsConfig, client *http.Client, rateLimiter *rate.Limiter, violationEvent *models.PolicyViolationEvent) error {
	// Apply rate limiting
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := rateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit exceeded: %w", err)
	}

	// Convert to JSON
	jsonData, err := json.Marshal(violationEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal violation event: %w", err)
	}

	url := fmt.Sprintf("%s/v0/organizations/%s/clusters/%s/data/watcher/policy-violations",
		insightsConfig.Hostname,
		insightsConfig.Organization,
		insightsConfig.Cluster)

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+insightsConfig.Token)

	watcherVersion := version.Version
	req.Header.Set("X-Fairwinds-Watcher-Version", watcherVersion)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("insights API returned status %d", resp.StatusCode)
	}

	slog.Info("Successfully sent blocked policy violation to Insights API",
		"policies", violationEvent.Policies,
		"success", violationEvent.Success,
		"blocked", violationEvent.Blocked,
		"namespace", violationEvent.Namespace,
		"resource", violationEvent.Name)

	return nil
}

func IsKyvernoPolicyViolation(responseCode int, message string) bool {
	if responseCode >= 400 && strings.Contains(message, "kyverno") &&
		strings.Contains(message, "blocked due to the following policies") {
		return true
	}
	return false
}

func IsValidatingPolicyViolation(responseCode int, message string) bool {
	if responseCode >= 400 && strings.Contains(message, "vpol") &&
		strings.Contains(message, "kyverno") {
		return true
	}
	return false
}

func IsValidatingAdmissionPolicyViolation(responseCode int, message string) bool {
	if responseCode >= 400 && strings.Contains(message, "ValidatingAdmissionPolicy") {
		return true
	}
	return false
}

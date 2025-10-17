package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
)

// ConsoleHandler prints events to console instead of sending to Insights
type ConsoleHandler struct {
	insightsConfig models.InsightsConfig
}

// NewConsoleHandler creates a new console handler
func NewConsoleHandler(insightsConfig models.InsightsConfig) *ConsoleHandler {
	return &ConsoleHandler{
		insightsConfig: insightsConfig,
	}
}

// Handle prints the event to console
func (h *ConsoleHandler) Handle(watchedEvent *event.WatchedEvent) error {
	// Print a nice header for the event
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Printf("🚨 POLICY VIOLATION EVENT DETECTED\n")
	fmt.Println(strings.Repeat("=", 80))

	// Basic event information
	fmt.Printf("📅 Timestamp: %s\n", time.Unix(watchedEvent.Timestamp, 0).Format(time.RFC3339))
	fmt.Printf("🏷️  Event Type: %s\n", watchedEvent.EventType)
	fmt.Printf("📦 Resource Type: %s\n", watchedEvent.ResourceType)
	fmt.Printf("🏠 Namespace: %s\n", watchedEvent.Namespace)
	fmt.Printf("📝 Name: %s\n", watchedEvent.Name)
	fmt.Printf("🆔 UID: %s\n", watchedEvent.UID)

	// Add Kubernetes eventTime if available
	if watchedEvent.EventTime != "" {
		fmt.Printf("⏰ Event Time: %s\n", watchedEvent.EventTime)
	}

	// Extract and display policy violation details
	if watchedEvent.Data != nil {
		fmt.Println("\n📋 Event Data:")

		// Show message if available
		if message, ok := watchedEvent.Data["message"].(string); ok {
			fmt.Printf("💬 Message: %s\n", message)
		}

		// Show reason if available
		if reason, ok := watchedEvent.Data["reason"].(string); ok {
			fmt.Printf("🔍 Reason: %s\n", reason)
		}

		// Show involved object if available
		if involvedObject, ok := watchedEvent.Data["involvedObject"].(map[string]interface{}); ok {
			fmt.Println("\n🎯 Involved Object:")
			if kind, ok := involvedObject["kind"].(string); ok {
				fmt.Printf("   Kind: %s\n", kind)
			}
			if name, ok := involvedObject["name"].(string); ok {
				fmt.Printf("   Name: %s\n", name)
			}
			if namespace, ok := involvedObject["namespace"].(string); ok {
				fmt.Printf("   Namespace: %s\n", namespace)
			}
		}

		// Show source if available (CloudWatch vs local)
		if source, ok := watchedEvent.Data["source"].(string); ok {
			fmt.Printf("📍 Source: %s\n", source)
		}
	}

	// Show metadata if available
	if len(watchedEvent.Metadata) > 0 {
		fmt.Println("\n🔧 Metadata:")
		for key, value := range watchedEvent.Metadata {
			fmt.Printf("   %s: %v\n", key, value)
		}
	}

	// Try to extract policy violation details
	violationEvent, err := h.extractPolicyViolation(watchedEvent)
	if err != nil {
		fmt.Printf("\n⚠️  Could not extract policy violation details: %v\n", err)
	} else {
		fmt.Println("\n🚨 Policy Violation Details:")
		fmt.Printf("📜 Policy Name: %s\n", violationEvent.PolicyName)
		fmt.Printf("📊 Policy Result: %s\n", violationEvent.PolicyResult)
		fmt.Printf("🚫 Blocked: %t\n", violationEvent.Blocked)
		fmt.Printf("💬 Message: %s\n", violationEvent.Message)

		if violationEvent.Blocked {
			fmt.Println("\n🔴 This is a BLOCKED policy violation that would be sent to Insights!")
		} else {
			fmt.Println("\n🟡 This is a non-blocked policy violation (would not be sent to Insights)")
		}
	}

	// Show full JSON for debugging
	fmt.Println("\n📄 Full Event JSON:")
	jsonData, err := json.MarshalIndent(watchedEvent, "", "  ")
	if err != nil {
		fmt.Printf("❌ Error marshaling JSON: %v\n", err)
	} else {
		fmt.Println(string(jsonData))
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("✅ Event processed successfully")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	return nil
}

// extractPolicyViolation extracts policy violation details from the event
func (h *ConsoleHandler) extractPolicyViolation(watchedEvent *event.WatchedEvent) (*models.PolicyViolationEvent, error) {
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

// parsePolicyMessage parses policy violation messages to extract policy name, result, and blocked status
func (h *ConsoleHandler) parsePolicyMessage(message string) (policyName, policyResult string, blocked bool, err error) {
	// This is the same parsing logic as the PolicyViolationHandler
	// Handle different message formats for policy violations

	// Format 1: "Pod default/nginx: [require-team-label] fail (blocked); validation error: ..."
	if strings.Contains(message, "] fail (blocked)") {
		parts := strings.Split(message, "] fail (blocked)")
		if len(parts) >= 1 {
			policyPart := parts[0]
			if strings.Contains(policyPart, "[") {
				policyName = strings.TrimSpace(strings.Split(policyPart, "[")[1])
				return policyName, "fail", true, nil
			}
		}
	}

	// Format 2: "Pod default/nginx: [require-team-label] warn validation warning: ..."
	if strings.Contains(message, "] warn") {
		parts := strings.Split(message, "] warn")
		if len(parts) >= 1 {
			policyPart := parts[0]
			if strings.Contains(policyPart, "[") {
				policyName = strings.TrimSpace(strings.Split(policyPart, "[")[1])
				return policyName, "warn", false, nil
			}
		}
	}

	// Format 3: "Pod default/nginx: [require-team-label] validation error ..."
	if strings.Contains(message, "] validation error") {
		parts := strings.Split(message, "] validation error")
		if len(parts) >= 1 {
			policyPart := parts[0]
			if strings.Contains(policyPart, "[") {
				policyName = strings.TrimSpace(strings.Split(policyPart, "[")[1])
				return policyName, "fail", false, nil
			}
		}
	}

	// Format 4: "policy disallow-host-path/disallow-host-path fail: ..."
	if strings.HasPrefix(message, "policy ") && strings.Contains(message, " fail:") {
		parts := strings.Split(message, " fail:")
		if len(parts) >= 1 {
			policyPart := strings.TrimPrefix(parts[0], "policy ")
			policyName = strings.TrimSpace(policyPart)
			return policyName, "fail", false, nil
		}
	}

	// Format 5: "policy require-labels/require-labels fail (blocked): ..."
	if strings.HasPrefix(message, "policy ") && strings.Contains(message, " fail (blocked):") {
		parts := strings.Split(message, " fail (blocked):")
		if len(parts) >= 1 {
			policyPart := strings.TrimPrefix(parts[0], "policy ")
			policyName = strings.TrimSpace(policyPart)
			return policyName, "fail", true, nil
		}
	}

	// Format 6: "policy security-context/security-context warn: ..."
	if strings.HasPrefix(message, "policy ") && strings.Contains(message, " warn:") {
		parts := strings.Split(message, " warn:")
		if len(parts) >= 1 {
			policyPart := strings.TrimPrefix(parts[0], "policy ")
			policyName = strings.TrimSpace(policyPart)
			return policyName, "warn", false, nil
		}
	}

	// Format 7: "Deployment default/nginx: [disallow-host-path] fail; ..."
	if strings.Contains(message, "] fail;") {
		parts := strings.Split(message, "] fail;")
		if len(parts) >= 1 {
			policyPart := parts[0]
			if strings.Contains(policyPart, "[") {
				policyName = strings.TrimSpace(strings.Split(policyPart, "[")[1])
				return policyName, "fail", false, nil
			}
		}
	}

	// Format 8: "Pod default/test: [require-labels] warn; ..."
	if strings.Contains(message, "] warn;") {
		parts := strings.Split(message, "] warn;")
		if len(parts) >= 1 {
			policyPart := parts[0]
			if strings.Contains(policyPart, "[") {
				policyName = strings.TrimSpace(strings.Split(policyPart, "[")[1])
				return policyName, "warn", false, nil
			}
		}
	}

	// Format 9: "Deployment default/nginx: [disallow-host-path] fail (blocked); ..."
	if strings.Contains(message, "] fail (blocked);") {
		parts := strings.Split(message, "] fail (blocked);")
		if len(parts) >= 1 {
			policyPart := parts[0]
			if strings.Contains(policyPart, "[") {
				policyName = strings.TrimSpace(strings.Split(policyPart, "[")[1])
				return policyName, "fail", true, nil
			}
		}
	}

	return "", "", false, fmt.Errorf("could not parse policy message format: %s", message)
}

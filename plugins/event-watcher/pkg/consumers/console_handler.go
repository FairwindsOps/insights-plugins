package consumers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/utils"
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
func (h *ConsoleHandler) Handle(watchedEvent *models.WatchedEvent) error {
	// Print a nice header for the event
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Printf("🚨 POLICY VIOLATION EVENT DETECTED\n")
	fmt.Println(strings.Repeat("=", 80))

	// Basic event information
	fmt.Printf("📅 Timestamp: %s\n", time.Unix(watchedEvent.Timestamp, 0).Format(time.RFC3339))
	fmt.Printf("🏷️  Event Type: %s\n", watchedEvent.EventType)
	fmt.Printf("📦 Kind: %s\n", watchedEvent.Kind)
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
		if involvedObject, ok := watchedEvent.Data["involvedObject"].(map[string]any); ok {
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
		fmt.Printf("📜 Policies: %s\n", violationEvent.Policies)
		fmt.Printf("🚫 Blocked: %t\n", violationEvent.Blocked)
		fmt.Printf("✅ Success: %t\n", violationEvent.Success)
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
func (h *ConsoleHandler) extractPolicyViolation(watchedEvent *models.WatchedEvent) (*models.PolicyViolationEvent, error) {
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

	policies := utils.ExtractPoliciesFromMessage(message)
	policyResult := watchedEvent.Metadata["policyResult"].(string)
	blocked := policyResult == "fail"

	return &models.PolicyViolationEvent{
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
		Blocked:   blocked,
		EventTime: watchedEvent.EventTime,
	}, nil
}

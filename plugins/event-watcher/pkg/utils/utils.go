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
	KyvernoPolicyViolationPrefix                    = "kyverno-policy-violation"
	ValidatingPolicyViolationPrefix                 = "validating-policy-violation"
	ValidatingAdmissionPolicyViolationPrefix        = "validating-admission-policy-violation"
	AuditOnlyAllowedValidatingAdmissionPolicyPrefix = "audit-only-vap"
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

func ExtractAuditOnlyAllowedValidatingAdmissionPoliciesFromMessage(annotations map[string]string) map[string]map[string]string {
	// "[{\"message\":\"failed expression: object.spec.replicas \\u003e= 5\",\"policy\":\"check-deployment-replicas\",\"binding\":\"check-deployment-replicas-binding\",\"expressionIndex\":0,\"validationActions\":[\"Audit\"]}]"
	validationFailure := annotations["validation.policy.admission.k8s.io/validation_failure"]
	policyName := "unknown"
	startIndex := strings.Index(validationFailure, "policy\":\"")
	if startIndex != -1 {
		endIndex := strings.Index(validationFailure, "\"")
		if endIndex != -1 {
			policyName = validationFailure[startIndex+len("policy:\"") : endIndex]
		}
	}
	return map[string]map[string]string{
		policyName: {
			policyName: validationFailure,
		},
	}
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

/*
From audit event sample: samples/audit-validating-admission-policy.json
Check if the audit event is a validating admission policy violation
*/
func IsValidatingAdmissionPolicyViolationAuditOnlyAllowEvent(annotations map[string]string) bool {
	decision := annotations["authorization.k8s.io/decision"]
	validationFailure := annotations["validation.policy.admission.k8s.io/validation_failure"]
	if decision == "allow" && validationFailure != "" {
		return true
	}
	return false
}

// CreateBlockedPolicyViolationEventFromAuditEvent creates a blocked policy violation event from an audit event
func CreateBlockedWatchedEventFromAuditEvent(auditEvent models.AuditEvent) *models.WatchedEvent {
	if !IsKyvernoPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) &&
		!IsValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) &&
		!IsValidatingAdmissionPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		return nil
	}
	policies := map[string]map[string]string{}
	if IsKyvernoPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		policies = ExtractPoliciesFromMessage(auditEvent.ResponseStatus.Message)
	} else if IsValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		policies = ExtractValidatingPoliciesFromMessage(auditEvent.ResponseStatus.Message)
	} else if IsValidatingAdmissionPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		policies = ExtractValidatingAdmissionPoliciesFromMessage(auditEvent.ResponseStatus.Message)
	}
	objectRef := auditEvent.ObjectRef
	namespace := objectRef.Namespace
	name := objectRef.Name
	resource := objectRef.Resource

	policyMessage := auditEvent.ResponseStatus.Message

	reason := "Allowed"
	if auditEvent.ResponseStatus.Code >= 400 {
		reason = "Blocked"
	}

	name = fmt.Sprintf("%s-%s-%s-%s", KyvernoPolicyViolationPrefix, resource, name, auditEvent.AuditID)
	if IsValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		name = fmt.Sprintf("%s-%s-%s-%s", ValidatingPolicyViolationPrefix, resource, name, auditEvent.AuditID)
	} else if IsValidatingAdmissionPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		name = fmt.Sprintf("%s-%s-%s-%s", ValidatingAdmissionPolicyViolationPrefix, resource, name, auditEvent.AuditID)
	}

	// Create the policy violation event
	violationEvent := &models.WatchedEvent{
		EventType:    models.EventTypeAdded,
		ResourceType: resource,
		Namespace:    namespace,
		Name:         name,
		UID:          auditEvent.AuditID,
		Timestamp:    auditEvent.StageTimestamp.Unix(),
		EventTime:    auditEvent.StageTimestamp.UTC().Format(time.RFC3339),
		Success:      false,
		Blocked:      true,
		Data: map[string]interface{}{
			"reason":  reason,
			"message": policyMessage,
			"source": map[string]interface{}{
				"component": "cloudwatch",
			},
			"involvedObject": objectRef,
			"firstTimestamp": auditEvent.RequestReceivedTimestamp.Format(time.RFC3339),
			"lastTimestamp":  auditEvent.StageTimestamp.Format(time.RFC3339),
			"metadata":       auditEvent.Annotations,
		},
		Metadata: map[string]interface{}{
			"audit_id":      auditEvent.AuditID,
			"policies":      policies,
			"resource_name": name,
			"namespace":     namespace,
			"action":        reason,
			"message":       policyMessage,
			"timestamp":     auditEvent.StageTimestamp.Format(time.RFC3339),
			"event_time":    auditEvent.StageTimestamp.Format(time.RFC3339),
		},
	}
	return violationEvent
}

// createWatchedEventFromPolicyViolationEvent creates a watched event from a policy violation event
func CreateBlockedWatchedEventFromPolicyViolationEvent(violation *models.PolicyViolationEventModel, eventChannel chan *models.WatchedEvent) error {
	slog.Info("Creating blocked watched event from policy violation event", "violation", violation)
	if violation == nil {
		slog.Info("Policy violation event is nil, skipping", "violation", violation)
		return fmt.Errorf("policy violation event is nil")
	}
	slog.Info("Creating blocked watched event from policy violation event",
		"policies", violation.Policies,
		"resource_name", violation.ResourceName,
		"namespace", violation.Namespace,
		"action", violation.Action,
		"audit_id", violation.AuditID,
		"metadata", violation.Metadata,
		"timestamp", violation.Timestamp)

	ts := violation.Timestamp
	if !violation.Timestamp.IsZero() {
		ts = violation.Timestamp
	}
	// Create a watched event from a policy violation event
	watchedEvent := &models.WatchedEvent{
		EventType: models.EventTypeAdded, ResourceType: violation.ResourceType,
		Namespace: violation.Namespace,
		Name:      violation.ResourceName,
		UID:       violation.AuditID,
		Timestamp: ts.Unix(),
		EventTime: ts.UTC().Format(time.RFC3339),
		Success:   false,
		Blocked:   true,
		Data: map[string]interface{}{
			"reason":   violation.Action,
			"type":     "Warning",
			"message":  violation.Message,
			"policies": violation.Policies,
			"source": map[string]interface{}{
				"component": "audit-log-handler",
			},
			"involvedObject": map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       violation.ResourceType,
				"name":       violation.ResourceName,
				"namespace":  violation.Namespace,
				"uid":        violation.AuditID,
			},
			"firstTimestamp": violation.Timestamp.Format(time.RFC3339),
			"lastTimestamp":  violation.Timestamp.Format(time.RFC3339),
			"count":          1,
			"metadata":       violation.Metadata,
		},
		Metadata: map[string]interface{}{
			"audit_id":      violation.AuditID,
			"metadata":      violation.Metadata,
			"policies":      violation.Policies,
			"resource_name": violation.ResourceName,
			"namespace":     violation.Namespace,
			"action":        violation.Action,
			"message":       violation.Message,
			"timestamp":     violation.Timestamp.Format(time.RFC3339),
			"event_time":    violation.Timestamp.Format(time.RFC3339),
		},
	}

	// Send the event to the event channel
	select {
	case eventChannel <- watchedEvent:
		slog.Info("Sent watched event",
			"policies", violation.Policies,
			"resource_name", violation.ResourceName)
		return nil
	default:
		slog.Info("Event channel full, dropping watched event",
			"policies", violation.Policies,
			"resource_name", violation.ResourceName)
		return fmt.Errorf("event channel full")
	}
}

func CreateValidatingAdmissionPolicyViolationAuditOnlyAllowEvent(auditEvent models.AuditEvent) *models.PolicyViolationEventModel {
	slog.Info("Creating validating admission policy violation audit only, allow event", "audit_event", auditEvent)
	if !IsValidatingAdmissionPolicyViolationAuditOnlyAllowEvent(auditEvent.Annotations) {
		return nil
	}
	policies := map[string]map[string]string{}
	policies = ExtractAuditOnlyAllowedValidatingAdmissionPoliciesFromMessage(auditEvent.Annotations)
	metadata := map[string]interface{}{
		"annotations": map[string]string(auditEvent.Annotations),
	}
	return &models.PolicyViolationEventModel{
		Timestamp:    auditEvent.RequestReceivedTimestamp,
		Policies:     policies,
		ResourceType: auditEvent.ObjectRef.Resource,
		ResourceName: auditEvent.ObjectRef.Name,
		Namespace:    auditEvent.ObjectRef.Namespace,
		User:         auditEvent.User.Username,
		Action:       auditEvent.ResponseStatus.Status,
		Message:      auditEvent.ResponseStatus.Message,
		AuditID:      auditEvent.AuditID,
		Metadata:     metadata,
	}
}

func CreateAuditOnlyAllowWatchedEventFromValidatingAdmissionPolicyViolation(policyViolationEvent *models.PolicyViolationEventModel, eventChannel chan *models.WatchedEvent) {
	slog.Info("Creating audit only allow watched event from validating admission policy violation", "audit_event", policyViolationEvent)
	if policyViolationEvent == nil {
		slog.Info("Policy violation event is nil, skipping", "policy_violation_event", policyViolationEvent)
		return
	}
	slog.Info("Creating audit only allow watched event from validating admission policy violation", "policies", policyViolationEvent.Policies, "resource_name", policyViolationEvent.ResourceName, "namespace", policyViolationEvent.Namespace, "action", policyViolationEvent.Action, "audit_id", policyViolationEvent.AuditID, "metadata", policyViolationEvent.Metadata, "timestamp", policyViolationEvent.Timestamp)

	watchedEvent := &models.WatchedEvent{
		EventType: models.EventTypeAdded, ResourceType: policyViolationEvent.ResourceType,
		Namespace: policyViolationEvent.Namespace,
		Name:      fmt.Sprintf("%s-%s-%s-%s", AuditOnlyAllowedValidatingAdmissionPolicyPrefix, policyViolationEvent.ResourceType, policyViolationEvent.ResourceName, policyViolationEvent.AuditID),
		UID:       policyViolationEvent.AuditID,
		Timestamp: policyViolationEvent.Timestamp.Unix(),
		EventTime: policyViolationEvent.Timestamp.Format(time.RFC3339),
		Success:   false,
		Blocked:   false,
	}

	// Send the event to the event channel
	select {
	case eventChannel <- watchedEvent:
		slog.Info("Sent watched event",
			"policies", policyViolationEvent.Policies,
			"resource_name", policyViolationEvent.ResourceName)
	default:
		slog.Info("Event channel full, dropping watched event",
			"policies", policyViolationEvent.Policies,
			"resource_name", policyViolationEvent.ResourceName)
	}
}

// createPolicyViolation creates a policy violation event from an audit event
func CreateBlockedPolicyViolationEvent(auditEvent models.AuditEvent) *models.PolicyViolationEventModel {
	if !IsKyvernoPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) && !IsValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) && !IsValidatingAdmissionPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		return nil
	}
	policies := map[string]map[string]string{}
	if IsKyvernoPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		slog.Info("Kyverno policy violation", "policies", policies, "audit_id", auditEvent.AuditID)
		policies = ExtractPoliciesFromMessage(auditEvent.ResponseStatus.Message)
	} else if IsValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		slog.Info("Validating policy violation", "policies", policies, "audit_id", auditEvent.AuditID)
		policies = ExtractValidatingPoliciesFromMessage(auditEvent.ResponseStatus.Message)
		slog.Info("Validating policy violation", "policies", policies, "audit_id", auditEvent.AuditID)
	} else if IsValidatingAdmissionPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		slog.Info("Validating admission policy violation", "policies", policies, "audit_id", auditEvent.AuditID)
		policies = ExtractValidatingAdmissionPoliciesFromMessage(auditEvent.ResponseStatus.Message)
		slog.Info("Validating admission policy violation", "policies", policies, "audit_id", auditEvent.AuditID)
	}
	return &models.PolicyViolationEventModel{
		Timestamp:    auditEvent.RequestReceivedTimestamp,
		Policies:     policies,
		ResourceType: auditEvent.ObjectRef.Resource,
		ResourceName: auditEvent.ObjectRef.Name,
		Namespace:    auditEvent.ObjectRef.Namespace,
		User:         auditEvent.User.Username,
		Action:       auditEvent.ResponseStatus.Status,
		Message:      auditEvent.ResponseStatus.Message,
		AuditID:      auditEvent.AuditID,
	}
}

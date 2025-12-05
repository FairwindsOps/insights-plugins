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

	"github.com/allegro/bigcache/v3"
	version "github.com/fairwindsops/insights-plugins/plugins/event-watcher"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
	"github.com/ghodss/yaml"
	"golang.org/x/time/rate"
	v1 "k8s.io/api/core/v1"
)

const (
	KyvernoPolicyViolationPrefix                    = "kyverno-policy-violation"
	ValidatingPolicyViolationPrefix                 = "vpol-violation"
	NamespacedValidatingPolicyViolationPrefix       = "nvpol-violation"
	ImageValidatingPolicyViolationPrefix            = "ivpol-violation"
	ValidatingAdmissionPolicyViolationPrefix        = "vap-violation"
	AuditOnlyAllowedValidatingAdmissionPolicyPrefix = "audit-only-vap"
	AuditOnlyClusterPolicyViolationPrefix           = "audit-only-cp"
)

var alreadyProcessedAuditIDs *bigcache.BigCache

func init() {
	var err error
	config := bigcache.DefaultConfig(60 * time.Minute)
	config.HardMaxCacheSize = 512 // 512MB
	alreadyProcessedAuditIDs, err = bigcache.New(context.Background(), config)
	if err != nil {
		panic(err)
	}
	slog.Info("Bigcache created", "size", alreadyProcessedAuditIDs.Len(), "hard_max_cache_size", config.HardMaxCacheSize)
}

func IsPolicyViolationAlreadyProcessed(uid string) bool {
	if value, err := alreadyProcessedAuditIDs.Get(uid); err == nil && value != nil {
		slog.Debug("Policy violation already processed, skipping", "policy_violation_id", uid)
		return true
	}
	return false
}

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
	if strings.Contains(message, "vpol.") && !strings.Contains(message, "nvpol.") && strings.Contains(message, "kyverno") && strings.Contains(message, "denied the request:") {
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

func ExtractNamespacedValidatingPoliciesFromMessage(message string) map[string]map[string]string {
	policyName := "unknown"
	// Example: admission webhook "nvpol.validate.kyverno.svc-fail" denied the request: Policy check-deployment-replicas failed: deployments must have at least 5 replicas for high availability
	if strings.Contains(message, "nvpol.") && strings.Contains(message, "kyverno") && strings.Contains(message, "denied the request:") {
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

func ExtractImageValidatingPoliciesFromMessage(message string) map[string]map[string]string {
	policyName := "unknown"
	// Example: admission webhook "ivpol.validate.kyverno.svc-fail" denied the request: Policy require-signed-images error: failed to evaluate policy: Get "https://untrusted.registry.io/v2/": dial tcp: lookup untrusted.registry.io on 10.96.0.10:53: no such host
	if strings.Contains(message, "ivpol.") && strings.Contains(message, "kyverno") && strings.Contains(message, "denied the request:") {
		startIndex := strings.Index(message, "denied the request: Policy") + len("denied the request: Policy")
		endIndex := strings.Index(message, " error:")
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
	startIndex := strings.Index(validationFailure, "\"policy\":") + len("\"policy\":")
	if startIndex != -1 {
		substring := string(validationFailure[startIndex:])
		endIndex := strings.Index(substring, ",")
		if endIndex != -1 {
			policyName = substring[:endIndex]
			policyName = strings.ReplaceAll(policyName, "\"", "")
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
	if value, err := alreadyProcessedAuditIDs.Get(violationEvent.UID); err == nil && value != nil {
		slog.Debug("Policy violation already processed, skipping", "policy_violation_id", violationEvent.UID)
		return nil
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

	err = alreadyProcessedAuditIDs.Set(violationEvent.UID, []byte("true"))
	if err != nil {
		slog.Warn("Failed to set audit ID in bigcache", "error", err, "audit_id", violationEvent.UID)
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
	if responseCode >= 400 && strings.Contains(message, "vpol.") &&
		!strings.Contains(message, "ivpol.") &&
		!strings.Contains(message, "nvpol.") &&
		strings.Contains(message, "kyverno") {
		return true
	}
	return false
}

func IsNamespacedValidatingPolicyViolation(responseCode int, message string) bool {
	if responseCode >= 400 && strings.Contains(message, "nvpol.") &&
		strings.Contains(message, "kyverno") {
		return true
	}
	return false
}

func IsImageValidatingPolicyViolation(responseCode int, message string) bool {
	if responseCode >= 400 && strings.Contains(message, "ivpol.") &&
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

func IsAuditOnlyClusterPolicyViolation(event v1.Event) bool {
	return (event.InvolvedObject.Kind == "ClusterPolicy" || event.InvolvedObject.Kind == "Policy") &&
		event.Reason == "PolicyViolation" &&
		event.Action == "Resource Passed"
}

// CreateBlockedPolicyViolationEventFromAuditEvent creates a blocked policy violation event from an audit event
func CreateBlockedWatchedEventFromAuditEvent(auditEvent models.AuditEvent) *models.WatchedEvent {
	if !IsKyvernoPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) &&
		!IsValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) &&
		!IsNamespacedValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) &&
		!IsImageValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) &&
		!IsValidatingAdmissionPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		return nil
	}
	policies := map[string]map[string]string{}
	if IsKyvernoPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		policies = ExtractPoliciesFromMessage(auditEvent.ResponseStatus.Message)
	} else if IsValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		policies = ExtractValidatingPoliciesFromMessage(auditEvent.ResponseStatus.Message)
	} else if IsNamespacedValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		policies = ExtractNamespacedValidatingPoliciesFromMessage(auditEvent.ResponseStatus.Message)
	} else if IsImageValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		policies = ExtractImageValidatingPoliciesFromMessage(auditEvent.ResponseStatus.Message)
	} else if IsValidatingAdmissionPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		policies = ExtractValidatingAdmissionPoliciesFromMessage(auditEvent.ResponseStatus.Message)
	}
	objectRef := auditEvent.ObjectRef
	policyMessage := auditEvent.ResponseStatus.Message
	resource, namespace, name := extractObjectRefFromMessage(policyMessage, &objectRef)

	reason := "Allowed"
	if auditEvent.ResponseStatus.Code >= 400 {
		reason = "Blocked"
	}

	name = fmt.Sprintf("%s-%s-%s-%s", KyvernoPolicyViolationPrefix, resource, name, auditEvent.AuditID)
	if IsValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		name = fmt.Sprintf("%s-%s-%s-%s", ValidatingPolicyViolationPrefix, resource, name, auditEvent.AuditID)
	} else if IsNamespacedValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		name = fmt.Sprintf("%s-%s-%s-%s", NamespacedValidatingPolicyViolationPrefix, resource, name, auditEvent.AuditID)
	} else if IsImageValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		name = fmt.Sprintf("%s-%s-%s-%s", ImageValidatingPolicyViolationPrefix, resource, name, auditEvent.AuditID)
	} else if IsValidatingAdmissionPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		name = fmt.Sprintf("%s-%s-%s-%s", ValidatingAdmissionPolicyViolationPrefix, resource, name, auditEvent.AuditID)
	}

	// Create the policy violation event
	violationEvent := &models.WatchedEvent{
		EventType: models.EventTypeAdded,
		Kind:      resource,
		Namespace: namespace,
		Name:      name,
		UID:       auditEvent.AuditID,
		Timestamp: auditEvent.StageTimestamp.Unix(),
		EventTime: auditEvent.StageTimestamp.UTC().Format(time.RFC3339),
		Success:   false,
		Blocked:   true,
		Data: map[string]any{
			"reason":  reason,
			"message": policyMessage,
			"source": map[string]any{
				"component": "cloudwatch",
			},
			"involvedObject": objectRef,
			"firstTimestamp": auditEvent.RequestReceivedTimestamp.Format(time.RFC3339),
			"lastTimestamp":  auditEvent.StageTimestamp.Format(time.RFC3339),
			"annotations":    auditEvent.Annotations,
		},
		Metadata: map[string]any{
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
	slog.Debug("Creating blocked watched event from policy violation event", "violation", violation)
	if violation == nil {
		slog.Info("Policy violation event is nil, skipping", "violation", violation)
		return fmt.Errorf("policy violation event is nil")
	}
	slog.Debug("Creating blocked watched event from policy violation event",
		"policies", violation.Policies,
		"api_group", violation.APIGroup,
		"resource_type", violation.ResourceType,
		"api_version", violation.APIVersion,
		"name", violation.Name,
		"namespace", violation.Namespace,
		"action", violation.Action,
		"audit_id", violation.AuditID,
		"annotations", violation.Annotations,
		"timestamp", violation.Timestamp)

	ts := violation.Timestamp
	if !violation.Timestamp.IsZero() {
		ts = time.Now()
	}
	// Create a watched event from a policy violation event
	watchedEvent := &models.WatchedEvent{
		EventType: models.EventTypeAdded,
		Kind:      violation.ResourceType,
		Namespace: violation.Namespace,
		Name:      violation.Name,
		UID:       violation.AuditID,
		Timestamp: ts.Unix(),
		EventTime: ts.UTC().Format(time.RFC3339),
		Success:   false,
		Blocked:   true,
		Data: map[string]any{
			"reason":   violation.Action,
			"type":     "Warning",
			"message":  violation.Message,
			"policies": violation.Policies,
			"source": map[string]any{
				"component": "audit-log-handler",
			},
			"involvedObject": map[string]any{
				"apiVersion":   violation.APIVersion,
				"apiGroup":     violation.APIGroup,
				"kind":         violation.ResourceType,
				"resourceType": violation.ResourceType,
				"name":         violation.Name,
				"namespace":    violation.Namespace,
				"uid":          violation.AuditID,
			},
			"firstTimestamp": violation.Timestamp.Format(time.RFC3339),
			"lastTimestamp":  violation.Timestamp.Format(time.RFC3339),
			"count":          1,
			"annotations":    violation.Annotations,
		},
		Metadata: map[string]any{
			"audit_id":      violation.AuditID,
			"annotations":   violation.Annotations,
			"policies":      violation.Policies,
			"resource_name": violation.Name,
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
		slog.Debug("Sent watched event",
			"policies", violation.Policies,
			"resource_name", violation.Name)
		return nil
	default:
		slog.Info("Event channel full, dropping watched event",
			"policies", violation.Policies,
			"resource_name", violation.Name)
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
	return &models.PolicyViolationEventModel{
		Timestamp:    auditEvent.RequestReceivedTimestamp,
		Policies:     policies,
		APIVersion:   auditEvent.ObjectRef.APIVersion,
		APIGroup:     auditEvent.ObjectRef.APIGroup,
		ResourceType: auditEvent.ObjectRef.Resource,
		Name:         auditEvent.ObjectRef.Name,
		Namespace:    auditEvent.ObjectRef.Namespace,
		User:         auditEvent.User.Username,
		Action:       auditEvent.ResponseStatus.Status,
		Message:      auditEvent.ResponseStatus.Message,
		AuditID:      auditEvent.AuditID,
		Annotations:  auditEvent.Annotations,
	}
}

func CreateAuditOnlyAllowWatchedEventFromValidatingAdmissionPolicyViolation(policyViolationEvent *models.PolicyViolationEventModel, eventChannel chan *models.WatchedEvent) {
	slog.Info("Creating audit only allow watched event from validating admission policy violation", "audit_event", policyViolationEvent)
	if policyViolationEvent == nil {
		slog.Info("Policy violation event is nil, skipping", "policy_violation_event", policyViolationEvent)
		return
	}
	slog.Info("Creating audit only allow watched event from validating admission policy violation", "policies", policyViolationEvent.Policies, "resource_name", policyViolationEvent.Name, "namespace", policyViolationEvent.Namespace, "action", policyViolationEvent.Action, "audit_id", policyViolationEvent.AuditID, "annotations", policyViolationEvent.Annotations, "timestamp", policyViolationEvent.Timestamp)

	watchedEvent := &models.WatchedEvent{
		EventType: models.EventTypeAdded,
		Kind:      policyViolationEvent.ResourceType,
		Namespace: policyViolationEvent.Namespace,
		Name:      fmt.Sprintf("%s-%s-%s-%s", AuditOnlyAllowedValidatingAdmissionPolicyPrefix, policyViolationEvent.ResourceType, policyViolationEvent.Name, policyViolationEvent.AuditID),
		UID:       policyViolationEvent.AuditID,
		Timestamp: policyViolationEvent.Timestamp.Unix(),
		EventTime: policyViolationEvent.Timestamp.Format(time.RFC3339),
		Success:   false,
		Blocked:   false,
		Data: map[string]any{
			"reason":  policyViolationEvent.Action,
			"message": policyViolationEvent.Message,
			"source": map[string]any{
				"component": "cloudwatch",
			},
			"involvedObject": map[string]any{
				"apiVersion": policyViolationEvent.APIVersion,
				"apiGroup":   policyViolationEvent.APIGroup,
				"kind":       policyViolationEvent.ResourceType,
				"name":       policyViolationEvent.Name,
				"namespace":  policyViolationEvent.Namespace,
				"uid":        policyViolationEvent.AuditID,
			},
			"firstTimestamp": policyViolationEvent.Timestamp.Format(time.RFC3339),
			"lastTimestamp":  policyViolationEvent.Timestamp.Format(time.RFC3339),
			"annotations":    policyViolationEvent.Annotations,
		},
		Metadata: map[string]any{
			"audit_id":      policyViolationEvent.AuditID,
			"policies":      policyViolationEvent.Policies,
			"resource_type": policyViolationEvent.ResourceType,
			"api_group":     policyViolationEvent.APIGroup,
			"api_version":   policyViolationEvent.APIVersion,
			"name":          policyViolationEvent.Name,
			"namespace":     policyViolationEvent.Namespace,
			"action":        policyViolationEvent.Action,
			"message":       policyViolationEvent.Message,
			"timestamp":     policyViolationEvent.Timestamp.Format(time.RFC3339),
			"event_time":    policyViolationEvent.Timestamp.Format(time.RFC3339),
		},
	}

	// Send the event to the event channel
	select {
	case eventChannel <- watchedEvent:
		slog.Info("Sent audit only watched event",
			"policies", policyViolationEvent.Policies,
			"api_group", policyViolationEvent.APIGroup,
			"resource_type", policyViolationEvent.ResourceType,
			"api_version", policyViolationEvent.APIVersion,
			"name", policyViolationEvent.Name)
	default:
		slog.Info("Event channel full, dropping watched event",
			"policies", policyViolationEvent.Policies,
			"resource_type", policyViolationEvent.ResourceType,
			"api_version", policyViolationEvent.APIVersion,
			"name", policyViolationEvent.Name)
	}
}

// createPolicyViolation creates a policy violation event from an audit event
func CreateBlockedPolicyViolationEvent(auditEvent models.AuditEvent) *models.PolicyViolationEventModel {
	if !IsKyvernoPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) && !IsValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) && !IsNamespacedValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) && !IsImageValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) && !IsValidatingAdmissionPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		return nil
	}
	policies := map[string]map[string]string{}
	objectRef := auditEvent.ObjectRef
	policyMessage := auditEvent.ResponseStatus.Message
	resource, namespace, name := extractObjectRefFromMessage(policyMessage, &objectRef)
	if IsKyvernoPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		slog.Debug("Kyverno policy violation", "policies", policies, "audit_id", auditEvent.AuditID)
		policies = ExtractPoliciesFromMessage(auditEvent.ResponseStatus.Message)
		name = fmt.Sprintf("%s-%s-%s-%s", KyvernoPolicyViolationPrefix, resource, name, auditEvent.AuditID)
	} else if IsValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		slog.Debug("Validating policy violation", "policies", policies, "audit_id", auditEvent.AuditID)
		policies = ExtractValidatingPoliciesFromMessage(auditEvent.ResponseStatus.Message)
		name = fmt.Sprintf("%s-%s-%s-%s", ValidatingPolicyViolationPrefix, resource, name, auditEvent.AuditID)
	} else if IsNamespacedValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		slog.Debug("Namespaced validating policy violation", "policies", policies, "audit_id", auditEvent.AuditID)
		policies = ExtractNamespacedValidatingPoliciesFromMessage(auditEvent.ResponseStatus.Message)
		name = fmt.Sprintf("%s-%s-%s-%s", NamespacedValidatingPolicyViolationPrefix, resource, name, auditEvent.AuditID)
	} else if IsImageValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		slog.Debug("Image validating policy violation", "policies", policies, "audit_id", auditEvent.AuditID)
		policies = ExtractImageValidatingPoliciesFromMessage(auditEvent.ResponseStatus.Message)
		name = fmt.Sprintf("%s-%s-%s-%s", ImageValidatingPolicyViolationPrefix, resource, name, auditEvent.AuditID)
	} else if IsValidatingAdmissionPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {
		slog.Debug("Validating admission policy violation", "policies", policies, "audit_id", auditEvent.AuditID)
		policies = ExtractValidatingAdmissionPoliciesFromMessage(auditEvent.ResponseStatus.Message)
		name = fmt.Sprintf("%s-%s-%s-%s", ValidatingAdmissionPolicyViolationPrefix, resource, name, auditEvent.AuditID)
		slog.Debug("Validating admission policy violation", "policies", policies, "audit_id", auditEvent.AuditID)
	}
	return &models.PolicyViolationEventModel{
		Timestamp:    auditEvent.RequestReceivedTimestamp,
		Policies:     policies,
		APIVersion:   objectRef.APIVersion,
		APIGroup:     objectRef.APIGroup,
		ResourceType: resource,
		Name:         name,
		Namespace:    namespace,
		User:         auditEvent.User.Username,
		Action:       auditEvent.ResponseStatus.Status,
		Message:      auditEvent.ResponseStatus.Message,
		AuditID:      auditEvent.AuditID,
	}
}

func ExtractAuditOnlyClusterPoliciesFromMessage(policyName, message string) map[string]map[string]string {
	// Deployment default/james1-deployment: [check-for-labels] fail; validation error: The label `abcapp.kubernetes.io/name` is required. rule check-for-labels failed at path /metadata/labels/abcapp.kubernetes.io/name/
	policies := map[string]map[string]string{}
	startIndex := strings.Index(message, "[")
	endIndex := strings.Index(message, "]")
	ruleName := policyName
	if startIndex != -1 && endIndex != -1 {
		ruleName = message[startIndex+1 : endIndex]
		ruleName = strings.TrimSpace(ruleName)
	}
	policies[policyName] = map[string]string{
		ruleName: message,
	}
	return policies
}

func extractObjectRefFromMessage(policyMessage string, objectRef *models.ObjectRef) (string, string, string) {
	resource := objectRef.Resource
	namespace := objectRef.Namespace
	name := objectRef.Name
	if objectRef.UID == "" {
		// some responses do not have the referenced object properly set, so we need to extract it from the message
		// example: "resource Pod/insights-agent/workloads-29372898-vt4pm was blocked due to the following policies"
		index := strings.Index(policyMessage, "denied the request:")

		if index != -1 {
			subText := policyMessage[index+len("denied the request:"):]
			if index != -1 {
				index = strings.Index(subText, "resource ")
				if index != -1 {
					subText = subText[index+len("resource "):]
					index = strings.Index(subText, "/")
					if index != -1 {
						resource = subText[:index]
						index = strings.Index(subText, "/")
						if index != -1 {
							subText = subText[index+1:]
							index = strings.Index(subText, "/")
							if index != -1 {
								namespace = subText[:index]
								index = strings.Index(subText, "/")
								if index != -1 {
									finalIndex := strings.Index(subText, " was blocked")
									if finalIndex != -1 {
										name = subText[index+1 : finalIndex]
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return resource, namespace, name
}

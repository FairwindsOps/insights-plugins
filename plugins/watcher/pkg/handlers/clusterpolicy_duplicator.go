package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
)

// ClusterPolicyDuplicatorHandler handles ClusterPolicy events and creates audit duplicates
type ClusterPolicyDuplicatorHandler struct {
	insightsConfig models.InsightsConfig
	dynamicClient  dynamic.Interface
}

// NewClusterPolicyDuplicatorHandler creates a new ClusterPolicy duplicator handler
func NewClusterPolicyDuplicatorHandler(config models.InsightsConfig, dynamicClient dynamic.Interface) *ClusterPolicyDuplicatorHandler {
	return &ClusterPolicyDuplicatorHandler{
		insightsConfig: config,
		dynamicClient:  dynamicClient,
	}
}

// Handle processes ClusterPolicy events and creates audit duplicates
func (h *ClusterPolicyDuplicatorHandler) Handle(watchedEvent *event.WatchedEvent) error {
	logFields := logrus.Fields{
		"event_type":    watchedEvent.EventType,
		"resource_type": watchedEvent.ResourceType,
		"namespace":     watchedEvent.Namespace,
		"name":          watchedEvent.Name,
	}

	logrus.WithFields(logFields).Info("Processing ClusterPolicy event")

	// Only process ClusterPolicy resources
	if watchedEvent.ResourceType != "ClusterPolicy" {
		return nil
	}

	// Process ADDED, MODIFIED, and DELETED events
	switch watchedEvent.EventType {
	case event.EventTypeAdded:
		return h.handlePolicyAdded(watchedEvent)
	case event.EventTypeModified:
		return h.handlePolicyModified(watchedEvent)
	case event.EventTypeDeleted:
		return h.handlePolicyDeleted(watchedEvent)
	default:
		logrus.WithFields(logFields).Debug("Unsupported event type for ClusterPolicy")
		return nil
	}
}

// handlePolicyAdded processes ADDED events for ClusterPolicy
func (h *ClusterPolicyDuplicatorHandler) handlePolicyAdded(watchedEvent *event.WatchedEvent) error {
	logFields := logrus.Fields{
		"event_type":    watchedEvent.EventType,
		"resource_type": watchedEvent.ResourceType,
		"namespace":     watchedEvent.Namespace,
		"name":          watchedEvent.Name,
	}

	logrus.WithFields(logFields).Info("Processing ClusterPolicy ADDED event")

	// Get the ClusterPolicy
	policy, err := h.getClusterPolicy(watchedEvent.Name)
	if err != nil {
		return fmt.Errorf("failed to get ClusterPolicy: %w", err)
	}

	// Check if this policy needs an audit duplicate
	if !h.needsAuditDuplicate(policy) {
		logrus.WithFields(logFields).Debug("Policy does not need audit duplicate")
		return nil
	}

	// Create audit duplicate
	if err := h.createAuditDuplicate(policy); err != nil {
		return fmt.Errorf("failed to create audit duplicate: %w", err)
	}

	logrus.WithFields(logFields).Info("Successfully created audit duplicate for ClusterPolicy")
	return nil
}

// handlePolicyModified processes MODIFIED events for ClusterPolicy
func (h *ClusterPolicyDuplicatorHandler) handlePolicyModified(watchedEvent *event.WatchedEvent) error {
	logFields := logrus.Fields{
		"event_type":    watchedEvent.EventType,
		"resource_type": watchedEvent.ResourceType,
		"namespace":     watchedEvent.Namespace,
		"name":          watchedEvent.Name,
	}

	logrus.WithFields(logFields).Info("Processing ClusterPolicy MODIFIED event")

	// Skip if this is already an audit policy
	if strings.HasSuffix(watchedEvent.Name, "-insights-audit") {
		logrus.WithFields(logFields).Debug("Skipping modification of audit policy")
		return nil
	}

	// Get the ClusterPolicy
	policy, err := h.getClusterPolicy(watchedEvent.Name)
	if err != nil {
		return fmt.Errorf("failed to get ClusterPolicy: %w", err)
	}

	// Check if this policy needs an audit duplicate
	if !h.needsAuditDuplicate(policy) {
		// If it doesn't need an audit duplicate anymore, delete the existing one
		auditPolicyName := watchedEvent.Name + "-insights-audit"
		if h.auditPolicyExists(auditPolicyName) {
			if err := h.deleteAuditPolicy(auditPolicyName); err != nil {
				logrus.WithError(err).WithFields(logFields).Warn("Failed to delete audit policy")
			}
		}
		return nil
	}

	// Update audit duplicate
	if err := h.updateAuditPolicy(policy); err != nil {
		return fmt.Errorf("failed to update audit policy: %w", err)
	}

	logrus.WithFields(logFields).Info("Successfully updated audit duplicate for ClusterPolicy")
	return nil
}

// handlePolicyDeleted processes DELETED events for ClusterPolicy
func (h *ClusterPolicyDuplicatorHandler) handlePolicyDeleted(watchedEvent *event.WatchedEvent) error {
	logFields := logrus.Fields{
		"event_type":    watchedEvent.EventType,
		"resource_type": watchedEvent.ResourceType,
		"namespace":     watchedEvent.Namespace,
		"name":          watchedEvent.Name,
	}

	logrus.WithFields(logFields).Info("Processing ClusterPolicy DELETED event")

	// Skip if this is already an audit policy
	if strings.HasSuffix(watchedEvent.Name, "-insights-audit") {
		logrus.WithFields(logFields).Debug("Skipping deletion of audit policy")
		return nil
	}

	auditPolicyName := watchedEvent.Name + "-insights-audit"

	// Check if audit policy exists
	if !h.auditPolicyExists(auditPolicyName) {
		logrus.WithFields(logFields).Debug("No audit policy to delete")
		return nil
	}

	// Delete audit policy
	if err := h.deleteAuditPolicy(auditPolicyName); err != nil {
		return fmt.Errorf("failed to delete audit policy: %w", err)
	}

	logrus.WithFields(logFields).Info("Successfully deleted audit duplicate for ClusterPolicy")
	return nil
}

// CheckExistingPolicies checks for existing ClusterPolicies and creates audit duplicates
func (h *ClusterPolicyDuplicatorHandler) CheckExistingPolicies() error {
	logrus.Info("Checking existing ClusterPolicies for audit duplicates")

	// Get all ClusterPolicies
	policies, err := h.getClusterPolicies()
	if err != nil {
		return fmt.Errorf("failed to get ClusterPolicies: %w", err)
	}

	createdCount := 0
	for _, policy := range policies {
		if h.needsAuditDuplicate(policy) {
			auditPolicyName := policy.GetName() + "-insights-audit"
			if !h.auditPolicyExists(auditPolicyName) {
				logrus.WithFields(logrus.Fields{
					"policy_name": policy.GetName(),
				}).Info("Creating audit duplicate for existing policy")

				if err := h.createAuditDuplicate(policy); err != nil {
					logrus.WithError(err).WithFields(logrus.Fields{
						"policy_name": policy.GetName(),
					}).Warn("Failed to create audit duplicate for existing policy")
					continue
				}
				createdCount++
			}
		}
	}

	logrus.WithFields(logrus.Fields{
		"created_audit_policies": createdCount,
		"processed_policies":     len(policies),
	}).Info("Completed checking existing ClusterPolicies")

	return nil
}

// getClusterPolicy retrieves a ClusterPolicy by name
func (h *ClusterPolicyDuplicatorHandler) getClusterPolicy(name string) (*unstructured.Unstructured, error) {
	gvr := schema.GroupVersionResource{
		Group:    "kyverno.io",
		Version:  "v1",
		Resource: "clusterpolicies",
	}

	policy, err := h.dynamicClient.Resource(gvr).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ClusterPolicy %s: %w", name, err)
	}

	return policy, nil
}

// getClusterPolicies retrieves all ClusterPolicies
func (h *ClusterPolicyDuplicatorHandler) getClusterPolicies() ([]*unstructured.Unstructured, error) {
	gvr := schema.GroupVersionResource{
		Group:    "kyverno.io",
		Version:  "v1",
		Resource: "clusterpolicies",
	}

	list, err := h.dynamicClient.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ClusterPolicies: %w", err)
	}

	var policies []*unstructured.Unstructured
	for i := range list.Items {
		policies = append(policies, &list.Items[i])
	}

	return policies, nil
}

// needsAuditDuplicate checks if a ClusterPolicy needs an audit duplicate
func (h *ClusterPolicyDuplicatorHandler) needsAuditDuplicate(policy *unstructured.Unstructured) bool {
	// Skip if this is already an audit policy (name ends with "-insights-audit")
	if strings.HasSuffix(policy.GetName(), "-insights-audit") {
		return false
	}

	// Check if validationFailureAction is "Enforce" (which means it blocks)
	validationFailureAction, found, err := unstructured.NestedString(policy.Object, "spec", "validationFailureAction")
	if err != nil || !found {
		return false
	}

	return validationFailureAction == "Enforce"
}

// auditPolicyExists checks if an audit policy already exists
func (h *ClusterPolicyDuplicatorHandler) auditPolicyExists(auditPolicyName string) bool {
	_, err := h.getClusterPolicy(auditPolicyName)
	return err == nil
}

// createAuditDuplicate creates an audit duplicate of the ClusterPolicy
func (h *ClusterPolicyDuplicatorHandler) createAuditDuplicate(policy *unstructured.Unstructured) error {
	// Create audit policy
	auditPolicy := h.createAuditPolicy(policy)

	// Create the audit policy
	_, err := h.createClusterPolicy(auditPolicy)
	if err != nil {
		return fmt.Errorf("failed to create audit policy: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"original_policy": policy.GetName(),
		"audit_policy":    auditPolicy.GetName(),
	}).Info("Created audit ClusterPolicy")

	return nil
}

// createAuditPolicy creates an audit version of the ClusterPolicy
func (h *ClusterPolicyDuplicatorHandler) createAuditPolicy(policy *unstructured.Unstructured) *unstructured.Unstructured {
	// Deep copy the original policy
	auditPolicy := policy.DeepCopy()

	// Update metadata
	auditPolicy.SetName(policy.GetName() + "-insights-audit")
	auditPolicy.SetResourceVersion("")
	auditPolicy.SetUID("")
	auditPolicy.SetCreationTimestamp(metav1.Time{})
	auditPolicy.SetGeneration(0)
	auditPolicy.SetOwnerReferences(nil) // Remove owner references

	// Add labels to identify this as an audit policy
	labels := auditPolicy.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["insights.fairwinds.com/audit-policy"] = "true"
	labels["insights.fairwinds.com/original-policy"] = policy.GetName()
	auditPolicy.SetLabels(labels)

	// Add annotations
	annotations := auditPolicy.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["insights.fairwinds.com/created-by"] = "insights-event-watcher"
	annotations["insights.fairwinds.com/original-policy"] = policy.GetName()
	auditPolicy.SetAnnotations(annotations)

	// Change validationFailureAction to "Audit"
	unstructured.SetNestedField(auditPolicy.Object, "Audit", "spec", "validationFailureAction")

	return auditPolicy
}

// createClusterPolicy creates a ClusterPolicy
func (h *ClusterPolicyDuplicatorHandler) createClusterPolicy(policy *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	gvr := schema.GroupVersionResource{
		Group:    "kyverno.io",
		Version:  "v1",
		Resource: "clusterpolicies",
	}

	createdPolicy, err := h.dynamicClient.Resource(gvr).Create(context.TODO(), policy, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create ClusterPolicy: %w", err)
	}

	return createdPolicy, nil
}

// updateAuditPolicy updates an existing audit policy to match the original policy
func (h *ClusterPolicyDuplicatorHandler) updateAuditPolicy(policy *unstructured.Unstructured) error {
	auditPolicyName := policy.GetName() + "-insights-audit"

	// Get the existing audit policy
	existingAuditPolicy, err := h.getClusterPolicy(auditPolicyName)
	if err != nil {
		return fmt.Errorf("failed to get existing audit policy: %w", err)
	}

	// Create updated audit policy spec
	newAuditPolicy := h.createAuditPolicy(policy)

	// Update the existing policy with new spec while preserving metadata
	unstructured.SetNestedField(existingAuditPolicy.Object, newAuditPolicy.Object["spec"], "spec")

	// Update the audit policy
	_, err = h.updateClusterPolicy(existingAuditPolicy)
	if err != nil {
		return fmt.Errorf("failed to update audit policy: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"original_policy": policy.GetName(),
		"audit_policy":    auditPolicyName,
	}).Info("Updated audit ClusterPolicy")

	return nil
}

// updateClusterPolicy updates a ClusterPolicy
func (h *ClusterPolicyDuplicatorHandler) updateClusterPolicy(policy *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	gvr := schema.GroupVersionResource{
		Group:    "kyverno.io",
		Version:  "v1",
		Resource: "clusterpolicies",
	}

	updatedPolicy, err := h.dynamicClient.Resource(gvr).Update(context.TODO(), policy, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update ClusterPolicy: %w", err)
	}

	return updatedPolicy, nil
}

// deleteAuditPolicy deletes an audit policy
func (h *ClusterPolicyDuplicatorHandler) deleteAuditPolicy(auditPolicyName string) error {
	gvr := schema.GroupVersionResource{
		Group:    "kyverno.io",
		Version:  "v1",
		Resource: "clusterpolicies",
	}

	err := h.dynamicClient.Resource(gvr).Delete(context.TODO(), auditPolicyName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete ClusterPolicy: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"audit_policy": auditPolicyName,
	}).Info("Deleted audit ClusterPolicy")

	return nil
}

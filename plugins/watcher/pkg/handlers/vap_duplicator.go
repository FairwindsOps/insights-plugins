package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
)

// VAPDuplicatorHandler handles ValidatingAdmissionPolicy events and creates audit duplicates
type VAPDuplicatorHandler struct {
	insightsConfig models.InsightsConfig
	kubeClient     kubernetes.Interface
}

// NewVAPDuplicatorHandler creates a new VAP duplicator handler
func NewVAPDuplicatorHandler(config models.InsightsConfig, kubeClient kubernetes.Interface) *VAPDuplicatorHandler {
	return &VAPDuplicatorHandler{
		insightsConfig: config,
		kubeClient:     kubeClient,
	}
}

// Handle processes ValidatingAdmissionPolicy events and creates audit duplicates
func (h *VAPDuplicatorHandler) Handle(watchedEvent *event.WatchedEvent) error {
	logFields := logrus.Fields{
		"event_type":    watchedEvent.EventType,
		"resource_type": watchedEvent.ResourceType,
		"namespace":     watchedEvent.Namespace,
		"name":          watchedEvent.Name,
	}

	logrus.WithFields(logFields).Info("Processing ValidatingAdmissionPolicy event")

	// Only process ValidatingAdmissionPolicy resources
	if watchedEvent.ResourceType != "ValidatingAdmissionPolicy" {
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
		return nil
	}

}

// handlePolicyAdded handles ADDED events for ValidatingAdmissionPolicy resources
func (h *VAPDuplicatorHandler) handlePolicyAdded(watchedEvent *event.WatchedEvent) error {
	logFields := logrus.Fields{
		"event_type":    watchedEvent.EventType,
		"resource_type": watchedEvent.ResourceType,
		"namespace":     watchedEvent.Namespace,
		"name":          watchedEvent.Name,
	}

	logrus.WithFields(logFields).Info("Processing ValidatingAdmissionPolicy ADDED event")

	// Extract the ValidatingAdmissionPolicy from the event
	vap, err := h.extractValidatingAdmissionPolicy(watchedEvent)
	if err != nil {
		return fmt.Errorf("failed to extract ValidatingAdmissionPolicy: %w", err)
	}

	// Check if this policy needs an audit duplicate
	if !h.needsAuditDuplicate(vap) {
		logrus.WithFields(logFields).Debug("Policy does not need audit duplicate")
		return nil
	}

	// Create audit duplicate
	if err := h.createAuditDuplicate(vap); err != nil {
		return fmt.Errorf("failed to create audit duplicate: %w", err)
	}

	logrus.WithFields(logFields).Info("Successfully created audit duplicate for ValidatingAdmissionPolicy")
	return nil
}

// handlePolicyModified handles MODIFIED events for ValidatingAdmissionPolicy resources
func (h *VAPDuplicatorHandler) handlePolicyModified(watchedEvent *event.WatchedEvent) error {
	logFields := logrus.Fields{
		"event_type":    watchedEvent.EventType,
		"resource_type": watchedEvent.ResourceType,
		"namespace":     watchedEvent.Namespace,
		"name":          watchedEvent.Name,
	}

	logrus.WithFields(logFields).Info("Processing ValidatingAdmissionPolicy MODIFIED event")

	// Skip if this is already an audit policy
	if strings.HasSuffix(watchedEvent.Name, "-insights-audit") {
		logrus.WithFields(logFields).Debug("Skipping modification of audit policy")
		return nil
	}

	// Extract the ValidatingAdmissionPolicy from the event
	vap, err := h.extractValidatingAdmissionPolicy(watchedEvent)
	if err != nil {
		return fmt.Errorf("failed to extract ValidatingAdmissionPolicy: %w", err)
	}

	auditPolicyName := vap.Name + "-insights-audit"

	// Check if audit policy exists
	if !h.auditPolicyExists(auditPolicyName) {
		// If audit policy doesn't exist, create it if needed
		if h.needsAuditDuplicate(vap) {
			if err := h.createAuditDuplicate(vap); err != nil {
				return fmt.Errorf("failed to create audit duplicate: %w", err)
			}
			logrus.WithFields(logFields).Info("Created audit duplicate for modified policy")
		}
		return nil
	}

	// Update existing audit policy
	if err := h.updateAuditPolicy(vap); err != nil {
		return fmt.Errorf("failed to update audit policy: %w", err)
	}

	logrus.WithFields(logFields).Info("Successfully updated audit duplicate for ValidatingAdmissionPolicy")
	return nil
}

// handlePolicyDeleted handles DELETED events for ValidatingAdmissionPolicy resources
func (h *VAPDuplicatorHandler) handlePolicyDeleted(watchedEvent *event.WatchedEvent) error {
	logFields := logrus.Fields{
		"event_type":    watchedEvent.EventType,
		"resource_type": watchedEvent.ResourceType,
		"namespace":     watchedEvent.Namespace,
		"name":          watchedEvent.Name,
	}

	logrus.WithFields(logFields).Info("Processing ValidatingAdmissionPolicy DELETED event")

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

	// Delete audit policy and its bindings
	if err := h.deleteAuditPolicy(auditPolicyName); err != nil {
		return fmt.Errorf("failed to delete audit policy: %w", err)
	}

	logrus.WithFields(logFields).Info("Successfully deleted audit duplicate for ValidatingAdmissionPolicy")
	return nil
}

// extractValidatingAdmissionPolicy extracts a ValidatingAdmissionPolicy from the watched event
func (h *VAPDuplicatorHandler) extractValidatingAdmissionPolicy(watchedEvent *event.WatchedEvent) (*admissionregistrationv1beta1.ValidatingAdmissionPolicy, error) {
	if watchedEvent.Data == nil {
		return nil, fmt.Errorf("event data is nil")
	}

	// Get the policy from the cluster using the name from the event
	policyName := watchedEvent.Name
	if policyName == "" {
		return nil, fmt.Errorf("policy name is empty")
	}

	// Fetch the policy from the cluster
	vap, err := h.kubeClient.AdmissionregistrationV1beta1().ValidatingAdmissionPolicies().Get(context.TODO(), policyName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ValidatingAdmissionPolicy %s: %w", policyName, err)
	}

	return vap, nil
}

// needsAuditDuplicate checks if a ValidatingAdmissionPolicy needs an audit duplicate
func (h *VAPDuplicatorHandler) needsAuditDuplicate(vap *admissionregistrationv1beta1.ValidatingAdmissionPolicy) bool {
	// Skip if this is already an audit policy (name ends with "-insights-audit")
	if strings.HasSuffix(vap.Name, "-insights-audit") {
		return false
	}

	// Check if there are any bindings with only Deny actions
	bindings, err := h.getPolicyBindings(vap.Name)
	if err != nil {
		logrus.WithError(err).WithField("policy_name", vap.Name).Warn("Failed to get policy bindings")
		return false
	}

	// Check if any binding has only Deny actions
	for _, binding := range bindings {
		if h.hasOnlyDenyActions(binding) {
			// Check if audit duplicate already exists
			auditPolicyName := vap.Name + "-insights-audit"
			if h.auditPolicyExists(auditPolicyName) {
				logrus.WithFields(logrus.Fields{
					"policy_name":       vap.Name,
					"audit_policy_name": auditPolicyName,
				}).Debug("Audit policy already exists")
				return false
			}
			return true
		}
	}

	return false
}

// getPolicyBindings retrieves all ValidatingAdmissionPolicyBindings for a given policy
func (h *VAPDuplicatorHandler) getPolicyBindings(policyName string) ([]admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding, error) {
	bindings, err := h.kubeClient.AdmissionregistrationV1beta1().ValidatingAdmissionPolicyBindings().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var matchingBindings []admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding
	for _, binding := range bindings.Items {
		if binding.Spec.PolicyName == policyName {
			matchingBindings = append(matchingBindings, binding)
		}
	}

	return matchingBindings, nil
}

// hasOnlyDenyActions checks if a binding has only Deny actions
func (h *VAPDuplicatorHandler) hasOnlyDenyActions(binding admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding) bool {
	if len(binding.Spec.ValidationActions) == 0 {
		return false
	}

	// Check if all actions are Deny
	for _, action := range binding.Spec.ValidationActions {
		if action != admissionregistrationv1beta1.Deny {
			return false
		}
	}

	return true
}

// auditPolicyExists checks if an audit policy already exists
func (h *VAPDuplicatorHandler) auditPolicyExists(auditPolicyName string) bool {
	_, err := h.kubeClient.AdmissionregistrationV1beta1().ValidatingAdmissionPolicies().Get(context.TODO(), auditPolicyName, metav1.GetOptions{})
	return err == nil
}

// createAuditDuplicate creates an audit duplicate of the ValidatingAdmissionPolicy
func (h *VAPDuplicatorHandler) createAuditDuplicate(vap *admissionregistrationv1beta1.ValidatingAdmissionPolicy) error {
	// Create audit policy
	auditPolicy := h.createAuditPolicy(vap)

	// Create the audit policy
	createdPolicy, err := h.kubeClient.AdmissionregistrationV1beta1().ValidatingAdmissionPolicies().Create(context.TODO(), auditPolicy, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create audit policy: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"original_policy": vap.Name,
		"audit_policy":    createdPolicy.Name,
	}).Info("Created audit ValidatingAdmissionPolicy")

	// Create audit bindings for each original binding
	bindings, err := h.getPolicyBindings(vap.Name)
	if err != nil {
		return fmt.Errorf("failed to get policy bindings: %w", err)
	}

	for _, binding := range bindings {
		if h.hasOnlyDenyActions(binding) {
			auditBinding := h.createAuditBinding(&binding, createdPolicy.Name)

			_, err := h.kubeClient.AdmissionregistrationV1beta1().ValidatingAdmissionPolicyBindings().Create(context.TODO(), auditBinding, metav1.CreateOptions{})
			if err != nil {
				logrus.WithError(err).WithFields(logrus.Fields{
					"original_binding": binding.Name,
					"audit_policy":     createdPolicy.Name,
				}).Warn("Failed to create audit binding")
				continue
			}

			logrus.WithFields(logrus.Fields{
				"original_binding": binding.Name,
				"audit_binding":    auditBinding.Name,
				"audit_policy":     createdPolicy.Name,
			}).Info("Created audit ValidatingAdmissionPolicyBinding")
		}
	}

	return nil
}

// createAuditPolicy creates an audit version of the ValidatingAdmissionPolicy
func (h *VAPDuplicatorHandler) createAuditPolicy(vap *admissionregistrationv1beta1.ValidatingAdmissionPolicy) *admissionregistrationv1beta1.ValidatingAdmissionPolicy {
	// Deep copy the original policy
	auditPolicy := vap.DeepCopy()

	// Update metadata
	auditPolicy.Name = vap.Name + "-insights-audit"
	auditPolicy.ResourceVersion = ""
	auditPolicy.UID = ""
	auditPolicy.CreationTimestamp = metav1.Time{}
	auditPolicy.Generation = 0

	// Add labels to identify this as an audit policy
	if auditPolicy.Labels == nil {
		auditPolicy.Labels = make(map[string]string)
	}
	auditPolicy.Labels["insights.fairwinds.com/audit-policy"] = "true"
	auditPolicy.Labels["insights.fairwinds.com/original-policy"] = vap.Name

	// Add annotations
	if auditPolicy.Annotations == nil {
		auditPolicy.Annotations = make(map[string]string)
	}
	auditPolicy.Annotations["insights.fairwinds.com/created-by"] = "insights-event-watcher"
	auditPolicy.Annotations["insights.fairwinds.com/original-policy"] = vap.Name

	return auditPolicy
}

// createAuditBinding creates an audit version of the ValidatingAdmissionPolicyBinding
func (h *VAPDuplicatorHandler) createAuditBinding(binding *admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding, auditPolicyName string) *admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding {
	// Deep copy the original binding
	auditBinding := binding.DeepCopy()

	// Update metadata
	auditBinding.Name = binding.Name + "-insights-audit"
	auditBinding.ResourceVersion = ""
	auditBinding.UID = ""
	auditBinding.CreationTimestamp = metav1.Time{}
	auditBinding.Generation = 0

	// Update policy name to point to audit policy
	auditBinding.Spec.PolicyName = auditPolicyName

	// Set validation actions to Audit only
	auditBinding.Spec.ValidationActions = []admissionregistrationv1beta1.ValidationAction{
		admissionregistrationv1beta1.Audit,
	}

	// Add labels to identify this as an audit binding
	if auditBinding.Labels == nil {
		auditBinding.Labels = make(map[string]string)
	}
	auditBinding.Labels["insights.fairwinds.com/audit-binding"] = "true"
	auditBinding.Labels["insights.fairwinds.com/original-binding"] = binding.Name

	// Add annotations
	if auditBinding.Annotations == nil {
		auditBinding.Annotations = make(map[string]string)
	}
	auditBinding.Annotations["insights.fairwinds.com/created-by"] = "insights-event-watcher"
	auditBinding.Annotations["insights.fairwinds.com/original-binding"] = binding.Name

	return auditBinding
}

// updateAuditPolicy updates an existing audit policy to match the original policy
func (h *VAPDuplicatorHandler) updateAuditPolicy(vap *admissionregistrationv1beta1.ValidatingAdmissionPolicy) error {
	auditPolicyName := vap.Name + "-insights-audit"

	// Check if the audit policy exists
	_, err := h.kubeClient.AdmissionregistrationV1beta1().ValidatingAdmissionPolicies().Get(context.TODO(), auditPolicyName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get existing audit policy: %w", err)
	}

	// Create updated audit policy
	updatedAuditPolicy := h.createAuditPolicy(vap)

	// Update the audit policy
	_, err = h.kubeClient.AdmissionregistrationV1beta1().ValidatingAdmissionPolicies().Update(context.TODO(), updatedAuditPolicy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update audit policy: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"original_policy": vap.Name,
		"audit_policy":    auditPolicyName,
	}).Info("Updated audit ValidatingAdmissionPolicy")

	// Update audit bindings
	bindings, err := h.getPolicyBindings(vap.Name)
	if err != nil {
		return fmt.Errorf("failed to get policy bindings: %w", err)
	}

	for _, binding := range bindings {
		if h.hasOnlyDenyActions(binding) {
			auditBindingName := binding.Name + "-insights-audit"

			// Check if audit binding exists
			_, err := h.kubeClient.AdmissionregistrationV1beta1().ValidatingAdmissionPolicyBindings().Get(context.TODO(), auditBindingName, metav1.GetOptions{})
			if err != nil {
				// Create audit binding if it doesn't exist
				auditBinding := h.createAuditBinding(&binding, auditPolicyName)
				_, err := h.kubeClient.AdmissionregistrationV1beta1().ValidatingAdmissionPolicyBindings().Create(context.TODO(), auditBinding, metav1.CreateOptions{})
				if err != nil {
					logrus.WithError(err).WithFields(logrus.Fields{
						"original_binding": binding.Name,
						"audit_binding":    auditBindingName,
					}).Warn("Failed to create audit binding during update")
					continue
				}
				logrus.WithFields(logrus.Fields{
					"original_binding": binding.Name,
					"audit_binding":    auditBindingName,
				}).Info("Created audit binding during update")
			} else {
				// Update existing audit binding
				auditBinding := h.createAuditBinding(&binding, auditPolicyName)
				_, err := h.kubeClient.AdmissionregistrationV1beta1().ValidatingAdmissionPolicyBindings().Update(context.TODO(), auditBinding, metav1.UpdateOptions{})
				if err != nil {
					logrus.WithError(err).WithFields(logrus.Fields{
						"original_binding": binding.Name,
						"audit_binding":    auditBindingName,
					}).Warn("Failed to update audit binding")
					continue
				}
				logrus.WithFields(logrus.Fields{
					"original_binding": binding.Name,
					"audit_binding":    auditBindingName,
				}).Info("Updated audit binding")
			}
		}
	}

	return nil
}

// deleteAuditPolicy deletes an audit policy and its bindings
func (h *VAPDuplicatorHandler) deleteAuditPolicy(auditPolicyName string) error {
	// Delete audit bindings first
	bindings, err := h.kubeClient.AdmissionregistrationV1beta1().ValidatingAdmissionPolicyBindings().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list bindings: %w", err)
	}

	for _, binding := range bindings.Items {
		if binding.Spec.PolicyName == auditPolicyName {
			err := h.kubeClient.AdmissionregistrationV1beta1().ValidatingAdmissionPolicyBindings().Delete(context.TODO(), binding.Name, metav1.DeleteOptions{})
			if err != nil {
				logrus.WithError(err).WithFields(logrus.Fields{
					"audit_binding": binding.Name,
					"audit_policy":  auditPolicyName,
				}).Warn("Failed to delete audit binding")
				continue
			}
			logrus.WithFields(logrus.Fields{
				"audit_binding": binding.Name,
				"audit_policy":  auditPolicyName,
			}).Info("Deleted audit binding")
		}
	}

	// Delete audit policy
	err = h.kubeClient.AdmissionregistrationV1beta1().ValidatingAdmissionPolicies().Delete(context.TODO(), auditPolicyName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete audit policy: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"audit_policy": auditPolicyName,
	}).Info("Deleted audit ValidatingAdmissionPolicy")

	return nil
}

// CheckExistingPolicies checks all existing ValidatingAdmissionPolicies and creates audit duplicates if needed
func (h *VAPDuplicatorHandler) CheckExistingPolicies() error {
	logrus.Info("Checking existing ValidatingAdmissionPolicies for audit duplicates")

	// Get all ValidatingAdmissionPolicies
	policies, err := h.kubeClient.AdmissionregistrationV1beta1().ValidatingAdmissionPolicies().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list ValidatingAdmissionPolicies: %w", err)
	}

	logrus.WithField("total_policies", len(policies.Items)).Info("Found ValidatingAdmissionPolicies")

	processedCount := 0
	createdCount := 0

	for _, policy := range policies.Items {
		// Skip if this is already an audit policy
		if strings.HasSuffix(policy.Name, "-insights-audit") {
			continue
		}

		processedCount++

		// Check if this policy needs an audit duplicate
		if h.needsAuditDuplicate(&policy) {
			logrus.WithField("policy_name", policy.Name).Info("Creating audit duplicate for existing policy")

			if err := h.createAuditDuplicate(&policy); err != nil {
				logrus.WithError(err).WithField("policy_name", policy.Name).Warn("Failed to create audit duplicate for existing policy")
				continue
			}

			createdCount++
		}
	}

	logrus.WithFields(logrus.Fields{
		"processed_policies":     processedCount,
		"created_audit_policies": createdCount,
	}).Info("Completed checking existing ValidatingAdmissionPolicies")

	return nil
}

package sync

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// PolicyManager handles the application and management of Kyverno policies
type PolicyManager struct {
	client        kubernetes.Interface
	dynamicClient dynamic.Interface
}

// NewPolicyManager creates a new policy manager
func NewPolicyManager(client kubernetes.Interface, dynamicClient dynamic.Interface) *PolicyManager {
	return &PolicyManager{
		client:        client,
		dynamicClient: dynamicClient,
	}
}

// ensureTempDir ensures the temporary directory exists
func ensureTempDir() error {
	tempDir := "/output/tmp"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory %s: %w", tempDir, err)
	}
	return nil
}

// validatePolicies is a no-op function - policies will be validated when applied
func (pm *PolicyManager) validatePolicies(ctx context.Context, policies []ClusterPolicy) error {
	if len(policies) == 0 {
		return nil
	}

	slog.Info("Skipping policy validation - policies will be validated during application", "count", len(policies))
	return nil
}

// executeDryRun performs a dry-run of the sync actions
func (p *PolicySyncProcessor) executeDryRun(ctx context.Context, actions PolicySyncActions, expectedPolicies []ClusterPolicy) (*PolicySyncResult, error) {
	slog.Info("Executing dry-run to validate sync plan")

	result := &PolicySyncResult{
		DryRun:  true,
		Actions: actions,
		Applied: actions.ToApply,
		Updated: actions.ToUpdate,
		Removed: actions.ToRemove,
		Failed:  []string{},
		Errors:  []string{},
		Success: true,
	}

	// Simulate policy operations
	for _, policyName := range actions.ToApply {
		slog.Info("[DRY-RUN] Would apply policy", "policy", policyName)
	}

	for _, policyName := range actions.ToUpdate {
		slog.Info("[DRY-RUN] Would update policy", "policy", policyName)
	}

	for _, policyName := range actions.ToRemove {
		slog.Info("[DRY-RUN] Would remove policy", "policy", policyName)
	}

	result.Summary = p.generateSummary(result)
	return result, nil
}

// executeSyncActions executes the actual sync actions
func (p *PolicySyncProcessor) executeSyncActions(ctx context.Context, actions PolicySyncActions, expectedPolicies []ClusterPolicy, result *PolicySyncResult) error {
	policyManager := NewPolicyManager(p.k8sClient, p.dynamicClient)

	// Create expected policies map for lookup
	expectedMap := make(map[string]ClusterPolicy)
	for _, policy := range expectedPolicies {
		expectedMap[policy.Name] = policy
	}

	// Apply new policies
	for _, policyName := range actions.ToApply {
		if policy, exists := expectedMap[policyName]; exists {
			if err := policyManager.applyPolicy(ctx, policy, p.config.DryRun); err != nil {
				result.Failed = append(result.Failed, policyName)
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to apply policy %s: %v", policyName, err))
				slog.Error("Failed to apply policy", "policy", policyName, "error", err)
			} else {
				result.Applied = append(result.Applied, policyName)
				slog.Info("Successfully applied policy", "policy", policyName)
			}
		}
	}

	// Update existing policies
	for _, policyName := range actions.ToUpdate {
		if policy, exists := expectedMap[policyName]; exists {
			if err := policyManager.updatePolicy(ctx, policy, p.config.DryRun); err != nil {
				result.Failed = append(result.Failed, policyName)
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to update policy %s: %v", policyName, err))
				slog.Error("Failed to update policy", "policy", policyName, "error", err)
			} else {
				result.Updated = append(result.Updated, policyName)
				slog.Info("Successfully updated policy", "policy", policyName)
			}
		}
	}

	// Remove orphaned policies
	for _, policyName := range actions.ToRemove {
		if err := policyManager.removePolicy(ctx, policyName, p.config.DryRun); err != nil {
			result.Failed = append(result.Failed, policyName)
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to remove policy %s: %v", policyName, err))
			slog.Error("Failed to remove policy", "policy", policyName, "error", err)
		} else {
			result.Removed = append(result.Removed, policyName)
			slog.Info("Successfully removed policy", "policy", policyName)
		}
	}

	return nil
}

// applyPolicy applies a new policy to the cluster using Kyverno CLI
func (pm *PolicyManager) applyPolicy(ctx context.Context, policy ClusterPolicy, dryRun bool) error {
	if dryRun {
		slog.Info("[DRY-RUN] Would apply policy", "policy", policy.Name)
		return nil
	}

	// Convert policy to YAML
	policyYAML, err := pm.policyToYAML(policy)
	if err != nil {
		return fmt.Errorf("failed to convert policy to YAML: %w", err)
	}

	// Ensure temp directory exists
	if err := ensureTempDir(); err != nil {
		return err
	}

	// Create temporary file for policy
	tempFile, err := os.CreateTemp("/output/tmp", "kyverno-policy-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	// Write policy to temporary file
	if _, err := tempFile.WriteString(policyYAML); err != nil {
		return fmt.Errorf("failed to write policy to temporary file: %w", err)
	}
	tempFile.Close()

	// Apply policy using Kyverno CLI
	cmd := exec.CommandContext(ctx, "kyverno", "apply", tempFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply policy %s with kyverno CLI: %s", policy.Name, string(output))
	}

	slog.Info("Successfully applied policy", "policy", policy.Name, "output", string(output))
	return nil
}

// updatePolicy updates an existing policy in the cluster using Kyverno CLI
func (pm *PolicyManager) updatePolicy(ctx context.Context, policy ClusterPolicy, dryRun bool) error {
	if dryRun {
		slog.Info("[DRY-RUN] Would update policy", "policy", policy.Name)
		return nil
	}

	// Convert policy to YAML
	policyYAML, err := pm.policyToYAML(policy)
	if err != nil {
		return fmt.Errorf("failed to convert policy to YAML: %w", err)
	}

	// Ensure temp directory exists
	if err := ensureTempDir(); err != nil {
		return err
	}

	// Create temporary file for policy
	tempFile, err := os.CreateTemp("/output/tmp", "kyverno-policy-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	// Write policy to temporary file
	if _, err := tempFile.WriteString(policyYAML); err != nil {
		return fmt.Errorf("failed to write policy to temporary file: %w", err)
	}
	tempFile.Close()

	// Update policy using Kyverno CLI (kyverno apply handles both create and update)
	cmd := exec.CommandContext(ctx, "kyverno", "apply", tempFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update policy %s with kyverno CLI: %s", policy.Name, string(output))
	}

	slog.Info("Successfully updated policy", "policy", policy.Name, "output", string(output))
	return nil
}

// removePolicy removes a policy from the cluster using Kyverno CLI
func (pm *PolicyManager) removePolicy(ctx context.Context, policyName string, dryRun bool) error {
	if dryRun {
		slog.Info("[DRY-RUN] Would remove policy", "policy", policyName)
		return nil
	}

	// Double-check the policy is still Insights-managed before deletion
	policy, err := pm.getPolicy(ctx, policyName)
	if err != nil {
		return fmt.Errorf("failed to get policy %s for deletion: %w", policyName, err)
	}

	// Verify it's still Insights-managed
	annotations := policy.GetAnnotations()
	if annotations == nil || annotations["insights.fairwinds.com/owned-by"] != "Fairwinds Insights" {
		slog.Warn("Policy is no longer Insights-managed, skipping deletion", "policy", policyName)
		return nil
	}

	// Delete the policy using Kyverno CLI
	cmd := exec.CommandContext(ctx, "kyverno", "delete", "clusterpolicy", policyName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete policy %s with kyverno CLI: %s", policyName, string(output))
	}

	slog.Info("Successfully removed policy", "policy", policyName, "output", string(output))
	return nil
}

// getPolicy retrieves a policy from the cluster
func (pm *PolicyManager) getPolicy(ctx context.Context, policyName string) (*unstructured.Unstructured, error) {
	return pm.dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "kyverno.io",
		Version:  "v1",
		Resource: "clusterpolicies",
	}).Get(ctx, policyName, metav1.GetOptions{})
}

// policyToUnstructured converts a ClusterPolicy to an unstructured object
func (pm *PolicyManager) policyToUnstructured(policy ClusterPolicy) (*unstructured.Unstructured, error) {
	policyMap := map[string]interface{}{
		"apiVersion": "kyverno.io/v1",
		"kind":       "ClusterPolicy",
		"metadata": map[string]interface{}{
			"name": policy.Name,
		},
		"spec": policy.Spec,
	}

	// Add annotations if they exist
	if len(policy.Annotations) > 0 {
		policyMap["metadata"].(map[string]interface{})["annotations"] = policy.Annotations
	}

	// Convert to unstructured
	policyObj := &unstructured.Unstructured{}
	policyObj.Object = policyMap

	return policyObj, nil
}

// policyToYAML converts a ClusterPolicy to YAML string
func (pm *PolicyManager) policyToYAML(policy ClusterPolicy) (string, error) {
	policyMap := map[string]interface{}{
		"apiVersion": "kyverno.io/v1",
		"kind":       "ClusterPolicy",
		"metadata": map[string]interface{}{
			"name": policy.Name,
		},
		"spec": policy.Spec,
	}

	// Add annotations if they exist
	if len(policy.Annotations) > 0 {
		policyMap["metadata"].(map[string]interface{})["annotations"] = policy.Annotations
	}

	// Convert to YAML
	yamlBytes, err := yaml.Marshal(policyMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal policy to YAML: %w", err)
	}

	return string(yamlBytes), nil
}

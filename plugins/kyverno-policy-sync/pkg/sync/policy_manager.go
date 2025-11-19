package sync

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

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

// executeDryRun performs a dry-run of the sync actions
func (p *PolicySyncProcessor) executeDryRun(ctx context.Context, actions PolicySyncActions) (*PolicySyncResult, error) {
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
func (p *PolicySyncProcessor) executeSyncActions(ctx context.Context, actions PolicySyncActions, managedPoliciesByInsights []ClusterPolicy, result *PolicySyncResult) error {
	policyManager := NewPolicyManager(p.k8sClient, p.dynamicClient)

	// Create managed policies by Insights map for lookup
	managedPoliciesByInsightsMap := make(map[string]ClusterPolicy)
	for _, policy := range managedPoliciesByInsights {
		managedPoliciesByInsightsMap[policy.Name] = policy
	}

	// Apply new policies
	for _, policyName := range actions.ToApply {
		if policy, exists := managedPoliciesByInsightsMap[policyName]; exists {
			if err := policyManager.applyPolicy(ctx, policy, p.config.DryRun); err != nil {
				result.Failed = append(result.Failed, policyName)
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to apply policy %s: %v", policyName, err))
				output := strings.Join(result.Errors, "\n")
				if !p.config.DryRun {
					err := p.insightsClient.UpdateKyvernoPolicyStatus(policyName, "failed", string(policy.YAML), output)
					if err != nil {
						slog.Error("Failed to update policy status in Insights", "error", err)
					}
				}
			} else {
				result.Applied = append(result.Applied, policyName)
				if !p.config.DryRun {
					err := p.insightsClient.UpdateKyvernoPolicyStatus(policyName, "success", string(policy.YAML), "")
					if err != nil {
						slog.Error("Failed to update policy status in Insights", "error", err)
					}
				}
			}
		}
	}

	// Update existing policies
	for _, policyName := range actions.ToUpdate {
		if policy, exists := managedPoliciesByInsightsMap[policyName]; exists {
			if err := policyManager.applyPolicy(ctx, policy, p.config.DryRun); err != nil {
				result.Failed = append(result.Failed, policyName)
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to update policy %s: %v", policyName, err))
				output := strings.Join(result.Errors, "\n")
				if !p.config.DryRun {
					err := p.insightsClient.UpdateKyvernoPolicyStatus(policyName, "failed", string(policy.YAML), output)
					if err != nil {
						slog.Error("Failed to update policy status in Insights", "error", err)
					}
				}
			} else {
				if !p.config.DryRun {
					err := p.insightsClient.UpdateKyvernoPolicyStatus(policyName, "success", string(policy.YAML), "")
					if err != nil {
						slog.Error("Failed to update policy status in Insights", "error", err)
					}
				}
			}
		}
	}

	// Remove orphaned policies
	for _, policyName := range actions.ToRemove {
		if err := policyManager.removePolicy(ctx, policyName, p.config.DryRun); err != nil {
			result.Failed = append(result.Failed, policyName)
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to remove policy %s: %v", policyName, err))
			slog.Error("Failed to remove policy", "policy", policyName, "error", err)
			output := strings.Join(result.Errors, "\n")
			if !p.config.DryRun {
				err := p.insightsClient.UpdateKyvernoPolicyStatus(policyName, "failed", "Failed to remove policy from cluster", output)
				if err != nil {
					slog.Error("Failed to update policy status in Insights", "error", err)
				}
			}
		} else {
			result.Removed = append(result.Removed, policyName)
			if !p.config.DryRun {
				err := p.insightsClient.UpdateKyvernoPolicyStatus(policyName, "success", "Successfully removed policy from cluster", "")
				if err != nil {
					slog.Error("Failed to update policy status in Insights", "error", err)
				}
			}
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
	if _, err := tempFile.Write(policy.YAML); err != nil {
		return fmt.Errorf("failed to write policy to temporary file: %w", err)
	}
	tempFile.Close()

	// Apply policy using kubectl
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", tempFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply policy %s with kubectl: %s: %s: %w", policy.Name, cmd.String(), string(output), err)
	}

	slog.Info("Successfully applied policy", "policy", policy.Name, "output", string(output))
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

	kind := policy.GetKind()
	if !validatePolicyKinds(kind) {
		slog.Warn("Invalid policy kind, skipping deletion", "policy", policyName, "kind", kind)
		return nil
	}

	cmd := exec.CommandContext(ctx, "kubectl", "delete", kind, policyName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete policy %s with kubectl: %s: %s: %w", policyName, cmd.String(), string(output), err)
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

// validatePolicyKinds validates the kind of a policy
func validatePolicyKinds(kind string) bool {
	validKinds := getPolicyKinds()
	for _, validKind := range validKinds {
		if kind == validKind {
			return true
		}
	}
	return false
}

func getPolicyKinds() []string {
	return []string{
		"ClusterPolicy",
		"Policy",
		"ValidatingPolicy",
		"ValidatingAdmissionPolicy",
	}
}

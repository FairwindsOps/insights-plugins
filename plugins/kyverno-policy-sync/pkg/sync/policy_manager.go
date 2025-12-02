package sync

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

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
	for _, policy := range actions.ToRemove {
		if err := policyManager.removePolicy(ctx, policy, p.config.DryRun); err != nil {
			result.Failed = append(result.Failed, policy.Name)
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to remove policy %s: %v", policy.Name, err))
			slog.Error("Failed to remove policy", "policy", policy.Name, "kind", policy.Kind, "error", err)
			output := strings.Join(result.Errors, "\n")
			if !p.config.DryRun {
				err := p.insightsClient.UpdateKyvernoPolicyStatus(policy.Name, "failed", "Failed to remove policy from cluster", output)
				if err != nil {
					slog.Error("Failed to update policy status in Insights", "error", err)
				}
			}
		} else {
			result.Removed = append(result.Removed, policy.Name)
			if !p.config.DryRun {
				err := p.insightsClient.UpdateKyvernoPolicyStatus(policy.Name, "success", "Successfully removed policy from cluster", "")
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
func (pm *PolicyManager) removePolicy(ctx context.Context, policy ClusterPolicy, dryRun bool) error {
	if dryRun {
		slog.Info("[DRY-RUN] Would remove policy", "policy", policy.Name, "kind", policy.Kind)
		return nil
	}

	cmd := exec.CommandContext(ctx, "kubectl", "delete", policy.Kind, policy.Name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete policy %s with kubectl: %s: %s: %w", policy.Name, cmd.String(), string(output), err)
	}

	slog.Info("Successfully removed policy", "policy", policy.Name, "kind", policy.Kind, "output", string(output))
	return nil
}

func getResourceConfigs() map[string]struct {
	group   string
	version string
} {
	return map[string]struct {
		group   string
		version string
	}{
		"clusterpolicies":              {group: "kyverno.io", version: "v1"},
		"policies":                     {group: "kyverno.io", version: "v1"},
		"validatingpolicies":           {group: "kyverno.io", version: "v1"},
		"validatingadmissionpolicies":  {group: "admissionregistration.k8s.io", version: "v1"},
		"clustercleanuppolicies":       {group: "kyverno.io", version: "v2"},
		"imagevalidatingpolicies":      {group: "policies.kyverno.io", version: "v1beta1"},
		"mutatingpolicies":             {group: "policies.kyverno.io", version: "v1beta1"},
		"generatingpolicies":           {group: "policies.kyverno.io", version: "v1beta1"},
		"deletingpolicies":             {group: "policies.kyverno.io", version: "v1beta1"},
		"namespacedvalidatingpolicies": {group: "policies.kyverno.io", version: "v1beta1"},
		"policyexceptions":             {group: "kyverno.io", version: "v1beta1"},
	}
}

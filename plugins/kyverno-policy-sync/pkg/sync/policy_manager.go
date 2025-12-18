package sync

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type PolicyManager interface {
	applyPolicy(ctx context.Context, policy ClusterPolicy) error
	removePolicy(ctx context.Context, policy ClusterPolicy) error
}

// PolicyManager handles the application and management of Kyverno policies
type DefaultPolicyManager struct {
}

// NewPolicyManager creates a new policy manager
func NewDefaultPolicyManager(client kubernetes.Interface, dynamicClient dynamic.Interface) PolicyManager {
	return &DefaultPolicyManager{}
}

// executeSyncActions executes the actual sync actions
func (p *PolicySyncProcessor) executeSyncActions(ctx context.Context, actions PolicySyncActions, managedPoliciesByInsights []ClusterPolicy, result *PolicySyncResult) error {

	// Create managed policies by Insights map for lookup
	managedPoliciesByInsightsMap := make(map[string]ClusterPolicy)
	for _, policy := range managedPoliciesByInsights {
		managedPoliciesByInsightsMap[policy.Name] = policy
	}

	// Apply new policies
	for _, policyName := range actions.ToApply {
		if policy, exists := managedPoliciesByInsightsMap[policyName]; exists {
			if err := p.policyManager.applyPolicy(ctx, policy); err != nil {
				errorMsg := fmt.Sprintf("Failed to apply policy %s: %v", policyName, err)
				result.Failed = append(result.Failed, policyName)
				result.Errors = append(result.Errors, errorMsg)
				err := p.insightsClient.UpdateKyvernoPolicyStatus(policyName, "apply", "failed", string(policy.YAML), errorMsg)
				if err != nil {
					slog.Error("Failed to update policy status in Insights", "error", err)
				}
			} else {
				result.Applied = append(result.Applied, policyName)
				err := p.insightsClient.UpdateKyvernoPolicyStatus(policyName, "apply", "success", string(policy.YAML), "")
				if err != nil {
					slog.Error("Failed to update policy status in Insights", "error", err)
				}
			}
		}
	}

	// Update existing policies
	for _, policyName := range actions.ToUpdate {
		if policy, exists := managedPoliciesByInsightsMap[policyName]; exists {
			if err := p.policyManager.applyPolicy(ctx, policy); err != nil {
				errorMsg := fmt.Sprintf("Failed to update policy %s: %v", policyName, err)
				result.Failed = append(result.Failed, policyName)
				result.Errors = append(result.Errors, errorMsg)
				err := p.insightsClient.UpdateKyvernoPolicyStatus(policyName, "apply", "failed", string(policy.YAML), errorMsg)
				if err != nil {
					slog.Error("Failed to update policy status in Insights", "error", err)
				}
			} else {
				err := p.insightsClient.UpdateKyvernoPolicyStatus(policyName, "apply", "success", string(policy.YAML), "")
				if err != nil {
					slog.Error("Failed to update policy status in Insights", "error", err)
				}
			}
		}
	}

	// Remove orphaned policies
	for _, policy := range actions.ToRemove {
		if err := p.policyManager.removePolicy(ctx, policy); err != nil {
			errorMsg := fmt.Sprintf("Failed to remove policy %s: %v", policy.Name, err)
			result.Failed = append(result.Failed, policy.Name)
			result.Errors = append(result.Errors, errorMsg)
			slog.Error("Failed to remove policy", "policy", policy.Name, "kind", policy.Kind, "error", err)
			err := p.insightsClient.UpdateKyvernoPolicyStatus(policy.Name, "delete", "failed", "Failed to remove policy from cluster", errorMsg)
			if err != nil {
				slog.Error("Failed to update policy status in Insights", "error", err)
			}
		} else {
			result.Removed = append(result.Removed, policy.Name)
			err := p.insightsClient.UpdateKyvernoPolicyStatus(policy.Name, "delete", "success", "Successfully removed policy from cluster", "")
			if err != nil {
				slog.Error("Failed to update policy status in Insights", "error", err)
			}
		}
	}
	return nil
}

// applyPolicy applies a new policy to the cluster using Kyverno CLI
func (pm DefaultPolicyManager) applyPolicy(ctx context.Context, policy ClusterPolicy) error {
	// Apply policy using kubectl with stdin
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	cmd.Stdin = bytes.NewReader(policy.YAML)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply policy %s with kubectl: %s: %s: %w", policy.Name, cmd.String(), string(output), err)
	}

	slog.Info("Successfully applied policy", "policy", policy.Name, "output", string(output))
	return nil
}

// removePolicy removes a policy from the cluster using Kyverno CLI
func (pm DefaultPolicyManager) removePolicy(ctx context.Context, policy ClusterPolicy) error {
	cmd := exec.CommandContext(ctx, "kubectl", "delete", policy.Kind, policy.Name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete policy %s with kubectl: %s: %s: %w", policy.Name, cmd.String(), string(output), err)
	}

	slog.Info("Successfully removed policy", "policy", policy.Name, "kind", policy.Kind, "output", string(output))
	return nil
}

// getResourceNames returns the list of resource names to discover
func getResourceNames() []string {
	return []string{
		"clusterpolicies",
		"policies",
		"validatingpolicies",
		"validatingadmissionpolicies",
		"clustercleanuppolicies",
		"imagevalidatingpolicies",
		"mutatingpolicies",
		"generatingpolicies",
		"deletingpolicies",
		"namespacedvalidatingpolicies",
		"policyexceptions",
	}
}

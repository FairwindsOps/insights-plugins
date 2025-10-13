package kyverno

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/FairwindsOps/insights-plugins/on-demand-job-runner/pkg/insights"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// PolicySyncProcessor handles the synchronization of Kyverno policies
type PolicySyncProcessor struct {
	insightsClient insights.Client
	k8sClient      kubernetes.Interface
	dynamicClient  dynamic.Interface
	config         PolicySyncConfig
	lock           *PolicySyncLock
}

// NewPolicySyncProcessor creates a new policy sync processor
func NewPolicySyncProcessor(insightsClient insights.Client, k8sClient kubernetes.Interface, dynamicClient dynamic.Interface, config PolicySyncConfig) *PolicySyncProcessor {
	return &PolicySyncProcessor{
		insightsClient: insightsClient,
		k8sClient:      k8sClient,
		dynamicClient:  dynamicClient,
		config:         config,
		lock: &PolicySyncLock{
			FilePath:    "/tmp/kyverno-policy-sync.lock",
			LockTimeout: config.LockTimeout,
		},
	}
}

// SyncPolicies performs the complete policy synchronization process
func (p *PolicySyncProcessor) SyncPolicies(ctx context.Context) (*PolicySyncResult, error) {
	startTime := time.Now()
	result := &PolicySyncResult{
		DryRun:  p.config.DryRun,
		Actions: PolicySyncActions{},
		Applied: []string{},
		Updated: []string{},
		Removed: []string{},
		Failed:  []string{},
		Errors:  []string{},
	}

	slog.Info("Starting Kyverno policy sync", "dryRun", p.config.DryRun)

	// Acquire lock to prevent concurrent sync operations
	if err := p.lock.Acquire(); err != nil {
		return result, fmt.Errorf("failed to acquire sync lock: %w", err)
	}
	defer p.lock.Release()

	// 1. Fetch expected policies from Insights API
	expectedPoliciesYAML, err := p.insightsClient.GetClusterKyvernoPoliciesYAML()
	if err != nil {
		return result, fmt.Errorf("failed to fetch expected policies from Insights: %w", err)
	}
	slog.Info("Fetched expected policies from Insights API", "yamlLength", len(expectedPoliciesYAML))

	// 2. Get currently deployed policies in cluster that are managed by Insights
	currentDeployedPolicies, err := p.listInsightsManagedPolicies(ctx)
	if err != nil {
		return result, fmt.Errorf("failed to list currently deployed Insights-managed policies: %w", err)
	}
	slog.Info("Found currently deployed Insights-managed policies", "count", len(currentDeployedPolicies))

	// 3. Parse expected policies from YAML
	expectedPolicies, err := p.parsePoliciesFromYAML(expectedPoliciesYAML)
	if err != nil {
		return result, fmt.Errorf("failed to parse expected policies from YAML: %w", err)
	}
	slog.Info("Parsed expected policies from YAML", "count", len(expectedPolicies))

	// 4. Compare policies and determine actions
	actions := p.comparePolicies(expectedPolicies, currentDeployedPolicies)
	result.Actions = actions

	slog.Info("Policy sync plan determined",
		"toApply", len(actions.ToApply),
		"toUpdate", len(actions.ToUpdate),
		"toRemove", len(actions.ToRemove))

	// 5. Validate policies if enabled
	if p.config.ValidatePolicies && !p.config.DryRun {
		policyManager := NewPolicyManager(p.k8sClient, p.dynamicClient)
		if err := policyManager.validatePolicies(ctx, expectedPolicies); err != nil {
			return result, fmt.Errorf("policy validation failed: %w", err)
		}
	}

	// 6. Execute dry-run first to check everything is right
	if !p.config.DryRun {
		dryRunResult, err := p.executeDryRun(ctx, actions, expectedPolicies)
		if err != nil {
			return result, fmt.Errorf("dry-run failed: %w", err)
		}
		slog.Info("Dry-run completed successfully", "summary", dryRunResult.Summary)
	}

	// 7. Execute sync actions
	if err := p.executeSyncActions(ctx, actions, expectedPolicies, result); err != nil {
		return result, fmt.Errorf("failed to execute sync actions: %w", err)
	}

	// 8. Generate summary
	result.Duration = time.Since(startTime)
	result.Summary = p.generateSummary(result)
	result.Success = len(result.Errors) == 0

	slog.Info("Kyverno policy sync completed",
		"success", result.Success,
		"duration", result.Duration,
		"summary", result.Summary)

	return result, nil
}

// listInsightsManagedPolicies lists all currently deployed policies managed by Insights
func (p *PolicySyncProcessor) listInsightsManagedPolicies(ctx context.Context) ([]ClusterPolicy, error) {
	// Get all ClusterPolicies from the cluster
	policies, err := p.dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "kyverno.io",
		Version:  "v1",
		Resource: "clusterpolicies",
	}).List(ctx, metav1.ListOptions{})

	if err != nil {
		return nil, fmt.Errorf("failed to list cluster policies: %w", err)
	}

	var insightsManagedPolicies []ClusterPolicy
	for _, item := range policies.Items {
		// Check if policy has Insights ownership annotation
		annotations := item.GetAnnotations()
		if annotations != nil && annotations["insights.fairwinds.com/owned-by"] == "Fairwinds Insights" {
			insightsManagedPolicies = append(insightsManagedPolicies, ClusterPolicy{
				Name:        item.GetName(),
				Annotations: annotations,
				Spec:        item.Object["spec"].(map[string]interface{}),
			})
		}
	}

	return insightsManagedPolicies, nil
}

// parsePoliciesFromYAML parses policies from YAML content
func (p *PolicySyncProcessor) parsePoliciesFromYAML(yamlContent string) ([]ClusterPolicy, error) {
	if strings.TrimSpace(yamlContent) == "" {
		return []ClusterPolicy{}, nil
	}

	// Split YAML documents
	documents := strings.Split(yamlContent, "---")
	var policies []ClusterPolicy

	for _, doc := range documents {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		// Parse YAML document
		var policy map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &policy); err != nil {
			slog.Warn("Failed to parse YAML document", "error", err, "document", doc)
			continue
		}

		// Extract policy name and metadata
		metadata, ok := policy["metadata"].(map[string]interface{})
		if !ok {
			slog.Warn("Policy missing metadata", "document", doc)
			continue
		}

		name, ok := metadata["name"].(string)
		if !ok {
			slog.Warn("Policy missing name in metadata", "document", doc)
			continue
		}

		// Extract annotations
		annotations := make(map[string]string)
		if ann, ok := metadata["annotations"].(map[string]interface{}); ok {
			for k, v := range ann {
				if str, ok := v.(string); ok {
					annotations[k] = str
				}
			}
		}

		policies = append(policies, ClusterPolicy{
			Name:        name,
			Annotations: annotations,
			Spec:        policy["spec"].(map[string]interface{}),
		})
	}

	return policies, nil
}

// comparePolicies compares expected policies with currently deployed policies
func (p *PolicySyncProcessor) comparePolicies(expected, current []ClusterPolicy) PolicySyncActions {
	actions := PolicySyncActions{
		ToApply:  []string{},
		ToUpdate: []string{},
		ToRemove: []string{},
	}

	// Create maps for efficient lookup
	expectedMap := make(map[string]ClusterPolicy)
	for _, policy := range expected {
		expectedMap[policy.Name] = policy
	}

	currentMap := make(map[string]ClusterPolicy)
	for _, policy := range current {
		currentMap[policy.Name] = policy
	}

	// Find policies to apply (new policies not yet deployed)
	for name := range expectedMap {
		if _, exists := currentMap[name]; !exists {
			actions.ToApply = append(actions.ToApply, name)
			slog.Debug("Policy will be applied", "policy", name, "reason", "new policy from Insights")
		}
	}

	// Find policies to update (existing deployed policies with changes)
	for name, expectedPolicy := range expectedMap {
		if currentPolicy, exists := currentMap[name]; exists {
			if !p.policiesEqual(expectedPolicy, currentPolicy) {
				actions.ToUpdate = append(actions.ToUpdate, name)
				slog.Debug("Policy will be updated", "policy", name, "reason", "changes detected")
			}
		}
	}

	// Find policies to remove (deployed Insights-managed policies not in expected list)
	for name := range currentMap {
		if _, exists := expectedMap[name]; !exists {
			actions.ToRemove = append(actions.ToRemove, name)
			slog.Debug("Policy will be removed", "policy", name, "reason", "no longer managed by Insights")
		}
	}

	return actions
}

// policiesEqual compares two policies for equality
func (p *PolicySyncProcessor) policiesEqual(expected, current ClusterPolicy) bool {
	// Simple comparison - in production, you might want more sophisticated comparison
	// For now, we'll do a basic spec comparison
	return fmt.Sprintf("%v", expected.Spec) == fmt.Sprintf("%v", current.Spec)
}

// generateSummary generates a human-readable summary of the sync operation
func (p *PolicySyncProcessor) generateSummary(result *PolicySyncResult) string {
	summary := fmt.Sprintf("Policy sync %s: ", map[bool]string{true: "completed", false: "failed"}[result.Success])

	if len(result.Applied) > 0 {
		summary += fmt.Sprintf("Applied %d, ", len(result.Applied))
	}
	if len(result.Updated) > 0 {
		summary += fmt.Sprintf("Updated %d, ", len(result.Updated))
	}
	if len(result.Removed) > 0 {
		summary += fmt.Sprintf("Removed %d, ", len(result.Removed))
	}
	if len(result.Failed) > 0 {
		summary += fmt.Sprintf("Failed %d, ", len(result.Failed))
	}

	summary += fmt.Sprintf("Duration: %v", result.Duration)

	return strings.TrimSuffix(summary, ", ")
}

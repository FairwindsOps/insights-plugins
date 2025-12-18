package sync

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/FairwindsOps/insights-plugins/kyverno-policy-sync/pkg/config"
	"github.com/FairwindsOps/insights-plugins/kyverno-policy-sync/pkg/insights"
	"github.com/FairwindsOps/insights-plugins/kyverno-policy-sync/pkg/lock"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// PolicySyncProcessor handles the synchronization of Kyverno policies
type PolicySyncProcessor struct {
	insightsClient insights.Client
	policyManager  PolicyManager
	dynamicClient  dynamic.Interface
	restMapper     meta.RESTMapper
	config         *config.Config
	lock           *lock.PolicySyncLock
}

// NewPolicySyncProcessor creates a new policy sync processor
func NewPolicySyncProcessor(insightsClient insights.Client, policyManager PolicyManager, lock *lock.PolicySyncLock, dynamicClient dynamic.Interface, restMapper meta.RESTMapper, config *config.Config) *PolicySyncProcessor {
	return &PolicySyncProcessor{
		insightsClient: insightsClient,
		policyManager:  policyManager,
		dynamicClient:  dynamicClient,
		restMapper:     restMapper,
		config:         config,
		lock:           lock,
	}
}

// SyncPolicies performs the complete policy synchronization process with leader election
func (p *PolicySyncProcessor) SyncPolicies(ctx context.Context) (*PolicySyncResult, error) {
	var result *PolicySyncResult
	var syncErr error

	// Run with leader election - the sync function will only execute when this instance becomes the leader
	err := p.lock.RunWithLeaderElection(ctx, func(leaderCtx context.Context) error {
		result, syncErr = p.syncPoliciesInternal(leaderCtx)
		return syncErr
	})

	if err != nil {
		return &PolicySyncResult{Success: false, Errors: []string{err.Error()}}, fmt.Errorf("leader election failed: %w", err)
	}

	if result == nil {
		return &PolicySyncResult{Success: false, Errors: []string{"sync result is nil"}}, fmt.Errorf("sync result is nil")
	}

	return result, syncErr
}

// syncPoliciesInternal performs the actual policy synchronization (called only by the leader)
func (p *PolicySyncProcessor) syncPoliciesInternal(ctx context.Context) (*PolicySyncResult, error) {
	startTime := time.Now()
	result := &PolicySyncResult{
		Actions: PolicySyncActions{},
		Applied: []string{},
		Updated: []string{},
		Removed: []string{},
		Failed:  []string{},
		Errors:  []string{},
	}

	slog.Info("Starting Kyverno policy sync", "dryRun", p.config.DryRun)

	// 1. Fetch expected policies from Insights API
	managedPoliciesByInsights, err := p.insightsClient.GetClusterKyvernoPoliciesYAML()
	if err != nil {
		return result, fmt.Errorf("failed to fetch expected policies from Insights: %w", err)
	}
	slog.Info("Fetched expected policies from Insights API", "yamlLength", len(managedPoliciesByInsights))

	// 2. Get currently deployed policies in cluster that are managed by Insights
	currentDeployedPolicies, err := p.listInsightsManagedPolicies(ctx)
	if err != nil {
		return result, fmt.Errorf("failed to list currently deployed Insights-managed policies: %w", err)
	}
	slog.Info("Found currently deployed Insights-managed policies", "count", len(currentDeployedPolicies))

	// 3. Parse expected policies from YAML
	parsedManagedPoliciesByInsights, err := parsePoliciesFromYAML(managedPoliciesByInsights)
	if err != nil {
		return result, fmt.Errorf("failed to parse expected policies from YAML: %w", err)
	}
	slog.Info("Parsed managed policies by Insights from YAML", "count", len(parsedManagedPoliciesByInsights))

	// 4. Compare policies and determine actions
	actions := p.comparePolicies(parsedManagedPoliciesByInsights, currentDeployedPolicies)
	result.Actions = actions

	slog.Info("Policy sync plan determined",
		"toApply", len(actions.ToApply),
		"toUpdate", len(actions.ToUpdate),
		"toRemove", len(actions.ToRemove))

	// 5. Execute sync actions
	if err := p.executeSyncActions(ctx, actions, parsedManagedPoliciesByInsights, result); err != nil {
		return result, fmt.Errorf("failed to execute sync actions: %w", err)
	}

	// 6. Generate summary
	result.Duration = time.Since(startTime)
	result.Success = len(result.Errors) == 0
	result.Summary = p.generateSummary(result)

	slog.Info("Kyverno policy sync completed",
		"success", result.Success,
		"duration", result.Duration,
		"summary", result.Summary)

	return result, nil
}

// listInsightsManagedPolicies lists all currently deployed policies managed by Insights
func (p *PolicySyncProcessor) listInsightsManagedPolicies(ctx context.Context) ([]ClusterPolicy, error) {
	var insightsManagedPolicies []ClusterPolicy
	for _, resourceName := range getResourceNames() {
		// Use RESTMapper to discover the correct group/version for the resource
		gvr, err := p.restMapper.ResourceFor(schema.GroupVersionResource{
			Resource: resourceName,
		})
		if err != nil {
			// Resource not found in the cluster - skip it
			errMsg := strings.ToLower(err.Error())
			if strings.Contains(errMsg, "not found") ||
				strings.Contains(errMsg, "could not find the requested resource") ||
				strings.Contains(errMsg, "no matches for") {
				slog.Debug("Resource not found in cluster, skipping", "resource", resourceName, "error", err.Error())
				continue
			}
			return nil, fmt.Errorf("failed to discover resource %s: %w", resourceName, err)
		}

		policies, err := p.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			// the server could not find the requested resource - we should continue if no resource is found
			if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "could not find the requested resource") {
				slog.Debug("Resource not found, skipping", "resource", resourceName)
				continue
			}
			return nil, fmt.Errorf("failed to list %s policies: %w", resourceName, err)
		}
		slog.Debug("Listed policies", "resource", resourceName, "count", len(policies.Items))

		// Process each policy found
		for _, item := range policies.Items {
			// Check if policy has Insights ownership annotation
			annotations := item.GetAnnotations()
			yaml, err := yaml.Marshal(item.Object)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal policy to YAML: %w", err)
			}
			policyName := item.GetName()
			hasAnnotation := annotations != nil && annotations["insights.fairwinds.com/owned-by"] == "Fairwinds Insights"
			slog.Debug("Checking policy", "name", policyName, "resource", resourceName, "hasAnnotation", hasAnnotation)

			if hasAnnotation {
				// Get the actual Kind from the object
				actualKind := item.GetKind()
				if actualKind == "" {
					// Fallback: derive kind from resource name
					actualKind = deriveKindFromResourceName(resourceName)
				}
				insightsManagedPolicies = append(insightsManagedPolicies, ClusterPolicy{
					Kind:        actualKind,
					Name:        policyName,
					Annotations: annotations,
					Spec:        item.Object["spec"].(map[string]any),
					YAML:        yaml,
				})
				slog.Debug("Added Insights-managed policy", "name", policyName, "kind", actualKind)
			}
		}
	}
	return insightsManagedPolicies, nil
}

// deriveKindFromResourceName converts a resource name (plural) to its Kind (singular, capitalized)
func deriveKindFromResourceName(resourceName string) string {
	switch resourceName {
	case "clusterpolicies":
		return "ClusterPolicy"
	case "policies":
		return "Policy"
	case "validatingpolicies":
		return "ValidatingPolicy"
	case "validatingadmissionpolicies":
		return "ValidatingAdmissionPolicy"
	case "clustercleanuppolicies":
		return "ClusterCleanupPolicy"
	case "imagevalidatingpolicies":
		return "ImageValidatingPolicy"
	case "mutatingpolicies":
		return "MutatingPolicy"
	case "generatingpolicies":
		return "GeneratingPolicy"
	case "deletingpolicies":
		return "DeletingPolicy"
	case "namespacedvalidatingpolicies":
		return "NamespacedValidatingPolicy"
	case "policyexceptions":
		return "PolicyException"
	default:
		return "ClusterPolicy" // default fallback
	}
}

// parsePoliciesFromYAML parses policies from YAML content
func parsePoliciesFromYAML(yamlContent string) ([]ClusterPolicy, error) {
	if strings.TrimSpace(yamlContent) == "" {
		return []ClusterPolicy{}, nil
	}

	decoder := yaml.NewDecoder(strings.NewReader(yamlContent))
	var policies []ClusterPolicy

	for {
		var policy map[string]any
		if err := decoder.Decode(&policy); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to parse YAML document: %w", err)
		}

		if len(policy) == 0 {
			continue
		}

		metadata, ok := policy["metadata"].(map[string]any)
		if !ok {
			slog.Warn("Policy missing metadata")
			continue
		}

		name, ok := metadata["name"].(string)
		if !ok {
			slog.Warn("Policy missing name in metadata")
			continue
		}

		kind, _ := policy["kind"].(string)
		if kind == "" {
			slog.Warn("Policy missing kind", "name", name)
			// Continue anyway, kind will be needed for deletion but not for application
		}

		// Extract annotations
		annotations := make(map[string]string)
		if ann, ok := metadata["annotations"].(map[string]any); ok {
			for k, v := range ann {
				if str, ok := v.(string); ok {
					annotations[k] = str
				}
			}
		}

		yamlBytes, err := yaml.Marshal(policy)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal policy to YAML: %w", err)
		}

		policies = append(policies, ClusterPolicy{
			Kind:        kind,
			Name:        name,
			YAML:        yamlBytes,
			Annotations: annotations,
			Spec:        policy["spec"].(map[string]any),
		})
	}

	return policies, nil
}

// comparePolicies compares expected policies with currently deployed policies
func (p *PolicySyncProcessor) comparePolicies(expected, current []ClusterPolicy) PolicySyncActions {
	actions := PolicySyncActions{
		ToApply:  []string{},
		ToUpdate: []string{},
		ToRemove: []ClusterPolicy{},
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
	for name := range expectedMap {
		if _, exists := currentMap[name]; exists {
			actions.ToUpdate = append(actions.ToUpdate, name)
			slog.Debug("Policy will be updated", "policy", name, "reason", "if any changes detected")
		}
	}

	// Find policies to remove (deployed Insights-managed policies not in expected list)
	for name := range currentMap {
		if _, exists := expectedMap[name]; !exists {
			// Store the full policy object so we have the kind information
			actions.ToRemove = append(actions.ToRemove, currentMap[name])
			slog.Debug("Policy will be removed", "policy", name, "kind", currentMap[name].Kind, "reason", "no longer managed by Insights")
		}
	}

	return actions
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

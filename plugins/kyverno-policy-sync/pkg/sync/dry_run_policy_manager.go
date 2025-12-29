package sync

import (
	"context"
	"log/slog"
)

type DryRunPolicyManager struct{}

func NewDryRunPolicyManager() PolicyManager {
	return &DryRunPolicyManager{}
}

func (d DryRunPolicyManager) applyPolicy(ctx context.Context, policy ClusterPolicy) error {
	slog.Info("[DRY-RUN] Would apply policy", "policy", policy.Name)
	return nil
}

func (d DryRunPolicyManager) removePolicy(ctx context.Context, policy ClusterPolicy) error {
	slog.Info("[DRY-RUN] Would remove policy", "policy", policy.Name, "kind", policy.Kind)
	return nil
}

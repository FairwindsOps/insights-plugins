package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/FairwindsOps/insights-plugins/kyverno-policy-sync/pkg/config"
	"github.com/FairwindsOps/insights-plugins/kyverno-policy-sync/pkg/insights"
	"github.com/FairwindsOps/insights-plugins/kyverno-policy-sync/pkg/k8s"
	"github.com/FairwindsOps/insights-plugins/kyverno-policy-sync/pkg/sync"
)

func main() {
	slog.Info("Starting Kyverno Policy Sync")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	slog.Info("Configuration loaded successfully",
		"organization", cfg.Organization,
		"cluster", cfg.Cluster,
		"host", cfg.Host,
		"token", strings.Repeat("*", len(cfg.Token)),
		"dryRun", cfg.DryRun,
		"lockTimeout", cfg.LockTimeout,
		"validatePolicies", cfg.ValidatePolicies)

	// Create clients
	insightsClient := insights.NewClient(cfg.Host, cfg.Token, cfg.Organization, cfg.Cluster, cfg.DevMode)

	k8sClient, err := k8s.GetClientSet()
	if err != nil {
		slog.Error("Failed to create Kubernetes client", "error", err)
		os.Exit(1)
	}

	dynamicClient, err := k8s.GetDynamicClient()
	if err != nil {
		slog.Error("Failed to create dynamic client", "error", err)
		os.Exit(1)
	}

	// Create sync configuration
	syncConfig := sync.PolicySyncConfig{
		DryRun:           cfg.DryRun,
		LockTimeout:      cfg.LockTimeout,
		ValidatePolicies: cfg.ValidatePolicies,
		Token:            cfg.Token,
	}

	// Create policy sync processor
	processor := sync.NewPolicySyncProcessor(insightsClient, k8sClient, dynamicClient, syncConfig)

	err = syncKyvernoPolicies(processor)
	if err != nil {
		slog.Error("Sync failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Sync completed successfully")
}

func syncKyvernoPolicies(processor *sync.PolicySyncProcessor) error {
	ctx := context.Background()
	result, err := processor.SyncPolicies(ctx)
	if err != nil {
		return err
	}

	slog.Info("Sync completed",
		"success", result.Success,
		"duration", result.Duration,
		"summary", result.Summary,
		"applied", len(result.Applied),
		"updated", len(result.Updated),
		"removed", len(result.Removed),
		"failed", len(result.Failed))

	if !result.Success {
		return fmt.Errorf("sync failed: %v", result.Errors)
	}
	return nil
}

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

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
		"token", maskToken(cfg.Token),
		"dryRun", cfg.DryRun,
		"syncInterval", cfg.SyncInterval,
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
		SyncInterval:     cfg.SyncInterval,
		LockTimeout:      cfg.LockTimeout,
		ValidatePolicies: cfg.ValidatePolicies,
	}

	// Create policy sync processor
	processor := sync.NewPolicySyncProcessor(insightsClient, k8sClient, dynamicClient, syncConfig)

	// Check if running in one-shot mode
	if os.Getenv("ONE_SHOT") == "true" {
		slog.Info("Running in one-shot mode")
		if err := runSync(processor); err != nil {
			slog.Error("Sync failed", "error", err)
			os.Exit(1)
		}
		slog.Info("Sync completed successfully")
		return
	}

	// Run in continuous mode
	slog.Info("Running in continuous mode", "interval", cfg.SyncInterval)
	runContinuous(processor, cfg.SyncInterval)
}

// runSync runs a single sync operation
func runSync(processor *sync.PolicySyncProcessor) error {
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

// runContinuous runs sync operations continuously
func runContinuous(processor *sync.PolicySyncProcessor, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run initial sync
	if err := runSync(processor); err != nil {
		slog.Error("Initial sync failed", "error", err)
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			if err := runSync(processor); err != nil {
				slog.Error("Sync failed", "error", err)
			}
		case sig := <-sigChan:
			slog.Info("Received signal, shutting down", "signal", sig)
			return
		}
	}
}

// maskToken masks the token for logging
func maskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "***" + token[len(token)-4:]
}

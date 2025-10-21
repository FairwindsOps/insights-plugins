package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/watcher"
	"github.com/spf13/cobra"
)

// auditCmd represents the audit command
var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Monitor local audit log files",
	Long: `Monitor local audit log files for policy violations.

This command reads Kubernetes audit log files from the local filesystem,
processes policy violations and sends them to Fairwinds Insights API.

Useful for kind clusters, local development, or any Kubernetes cluster
where audit logs are written to local files.`,
	RunE: runAudit,
}

var (
	// Audit specific flags
	auditLogPath string
)

func init() {
	RootCmd.AddCommand(auditCmd)

	// Audit specific flags
	auditCmd.Flags().StringVar(&auditLogPath, "log-path", "", "Path to audit log file (required)")

	// Mark required flags
	auditCmd.MarkFlagRequired("log-path")
}

func runAudit(cmd *cobra.Command, args []string) error {
	slog.Info("Starting audit log watcher",
		"log_path", auditLogPath)

	organizationName := os.Getenv("FAIRWINDS_ORGANIZATION")
	clusterName := os.Getenv("FAIRWINDS_CLUSTER")
	if organizationName == "" {
		return fmt.Errorf("FAIRWINDS_ORGANIZATION environment variable not set")
	}
	if clusterName == "" {
		return fmt.Errorf("FAIRWINDS_CLUSTER environment variable not set")
	}

	// Create insights config
	insightsConfig := models.InsightsConfig{
		Hostname:     insightsHost,
		Organization: organizationName,
		Cluster:      clusterName,
		Token:        getInsightsToken(consoleMode),
	}

	// Create watcher
	watcher, err := watcher.NewWatcher(
		insightsConfig,
		"local",      // logSource
		auditLogPath, // auditLogPath
		nil,          // cloudwatchConfig (not used for audit logs)
		eventBufferSize,
		httpTimeoutSeconds,
		rateLimitPerMinute,
		consoleMode,
	)
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		slog.Info("Received shutdown signal, stopping watcher...")
		cancel()
	}()

	// Start watcher
	if err := watcher.Start(ctx); err != nil {
		return fmt.Errorf("failed to start watcher: %w", err)
	}

	slog.Info("Audit log watcher started successfully",
		"active_sources", watcher.GetEventSourceCount(),
		"source_names", watcher.GetEventSourceNames())

	// Wait for shutdown signal
	<-ctx.Done()

	// Stop watcher
	watcher.Stop(ctx)
	slog.Info("Audit log watcher stopped")

	return nil
}

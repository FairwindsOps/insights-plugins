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

// cloudwatchCmd represents the cloudwatch command
var cloudwatchCmd = &cobra.Command{
	Use:   "cloudwatch",
	Short: "Monitor EKS audit logs from CloudWatch",
	Long: `Monitor EKS audit logs from CloudWatch for policy violations.

This command connects to AWS CloudWatch and monitors the specified log group
for Kubernetes audit events, processing policy violations and sending them
to Fairwinds Insights API.`,
	RunE: runCloudWatch,
}

var (
	// CloudWatch specific flags
	logGroup      string
	filterPattern string
	region        string
	startTime     string
	endTime       string
)

func init() {
	RootCmd.AddCommand(cloudwatchCmd)

	// CloudWatch specific flags
	cloudwatchCmd.Flags().StringVar(&logGroup, "log-group", "", "CloudWatch log group name (required)")
	cloudwatchCmd.Flags().StringVar(&filterPattern, "filter-pattern", "", "CloudWatch filter pattern")
	cloudwatchCmd.Flags().StringVar(&region, "region", "", "AWS region (required)")
	cloudwatchCmd.Flags().StringVar(&startTime, "start-time", "", "Start time for log query (RFC3339 format)")
	cloudwatchCmd.Flags().StringVar(&endTime, "end-time", "", "End time for log query (RFC3339 format)")

	// Mark required flags
	cloudwatchCmd.MarkFlagRequired("log-group")
	cloudwatchCmd.MarkFlagRequired("region")
}

func runCloudWatch(cmd *cobra.Command, args []string) error {
	slog.Info("Starting CloudWatch event watcher",
		"log_group", logGroup,
		"region", region,
		"filter_pattern", filterPattern)

	// Create insights config
	insightsConfig := models.InsightsConfig{
		Hostname: insightsHost,
		Token:    getInsightsToken(consoleMode),
	}

	// Create CloudWatch config
	cloudwatchConfig := &models.CloudWatchConfig{
		LogGroupName:  logGroup,
		FilterPattern: filterPattern,
		Region:        region,
	}

	// Create watcher
	watcher, err := watcher.NewWatcher(
		insightsConfig,
		"cloudwatch", // logSource
		"",           // auditLogPath (not used for CloudWatch)
		cloudwatchConfig,
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

	slog.Info("CloudWatch watcher started successfully",
		"active_sources", watcher.GetEventSourceCount(),
		"source_names", watcher.GetEventSourceNames())

	// Wait for shutdown signal
	<-ctx.Done()

	// Stop watcher
	watcher.Stop(ctx)
	slog.Info("CloudWatch watcher stopped")

	return nil
}

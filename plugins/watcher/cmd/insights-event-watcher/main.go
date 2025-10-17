package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/health"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/watcher"
)

// validateConfiguration validates all command-line parameters
func validateConfiguration(logLevel, insightsHost, organization, cluster, auditLogPath, logSource, cloudwatchLogGroup, cloudwatchRegion, cloudwatchFilter string, eventBufferSize, httpTimeoutSeconds, rateLimitPerMinute, cloudwatchBatchSize, cloudwatchMaxMemory, healthPort int, consoleMode bool) error {
	// Validate log level
	validLogLevels := []string{"debug", "info", "warn", "error"}
	logLevelValid := false
	for _, valid := range validLogLevels {
		if logLevel == valid {
			logLevelValid = true
			break
		}
	}
	if !logLevelValid {
		return fmt.Errorf("invalid log level '%s', must be one of: %s", logLevel, strings.Join(validLogLevels, ", "))
	}

	// Validate insights host URL if provided
	if insightsHost != "" {
		if !strings.HasPrefix(insightsHost, "http://") && !strings.HasPrefix(insightsHost, "https://") {
			insightsHost = "https://" + insightsHost
		}
		if _, err := url.Parse(insightsHost); err != nil {
			return fmt.Errorf("invalid insights host URL '%s': %w", insightsHost, err)
		}
	}

	// Validate organization name if provided
	if organization != "" {
		if strings.TrimSpace(organization) == "" {
			return fmt.Errorf("organization name cannot be empty or whitespace only")
		}
		if len(organization) > 64 {
			return fmt.Errorf("organization name too long (max 100 characters)")
		}
	}

	// Validate cluster name if provided
	if cluster != "" {
		if strings.TrimSpace(cluster) == "" {
			return fmt.Errorf("cluster name cannot be empty or whitespace only")
		}
		if len(cluster) > 64 {
			return fmt.Errorf("cluster name too long (max 100 characters)")
		}
	}

	// Note: Token validation is now handled via environment variables

	// Validate audit log path if provided
	if auditLogPath != "" {
		if strings.TrimSpace(auditLogPath) == "" {
			return fmt.Errorf("audit log path cannot be empty or whitespace only")
		}
		// Check if file exists and is readable
		if _, err := os.Stat(auditLogPath); err != nil {
			return fmt.Errorf("audit log path '%s' is not accessible: %w", auditLogPath, err)
		}
	}

	// Validate event buffer size
	if eventBufferSize < 1 {
		return fmt.Errorf("event buffer size must be at least 1, got %d", eventBufferSize)
	}
	if eventBufferSize > 10000 {
		return fmt.Errorf("event buffer size too large (max 10000), got %d", eventBufferSize)
	}

	// Validate HTTP timeout
	if httpTimeoutSeconds < 1 {
		return fmt.Errorf("HTTP timeout must be at least 1 second, got %d", httpTimeoutSeconds)
	}
	if httpTimeoutSeconds > 300 {
		return fmt.Errorf("HTTP timeout too large (max 300 seconds), got %d", httpTimeoutSeconds)
	}

	// Validate rate limit
	if rateLimitPerMinute < 1 {
		return fmt.Errorf("rate limit must be at least 1 call per minute, got %d", rateLimitPerMinute)
	}
	if rateLimitPerMinute > 1000 {
		return fmt.Errorf("rate limit too high (max 1000 calls per minute), got %d", rateLimitPerMinute)
	}

	// Validate log source
	validLogSources := []string{"local", "cloudwatch"}
	logSourceValid := false
	for _, valid := range validLogSources {
		if logSource == valid {
			logSourceValid = true
			break
		}
	}
	if !logSourceValid {
		return fmt.Errorf("invalid log source '%s', must be one of: %s", logSource, strings.Join(validLogSources, ", "))
	}

	// Validate CloudWatch configuration if log source is cloudwatch
	if logSource == "cloudwatch" {
		if cloudwatchLogGroup == "" {
			return fmt.Errorf("cloudwatch-log-group is required when log-source is cloudwatch")
		}
		if cloudwatchRegion == "" {
			return fmt.Errorf("cloudwatch-region is required when log-source is cloudwatch")
		}
		if !strings.HasPrefix(cloudwatchLogGroup, "/aws/eks/") {
			return fmt.Errorf("cloudwatch-log-group should start with '/aws/eks/', got '%s'", cloudwatchLogGroup)
		}

		// Validate CloudWatch batch size
		if cloudwatchBatchSize < 1 {
			return fmt.Errorf("cloudwatch batch size must be at least 1, got %d", cloudwatchBatchSize)
		}
		if cloudwatchBatchSize > 10000 {
			return fmt.Errorf("cloudwatch batch size too large (max 10000), got %d", cloudwatchBatchSize)
		}

		// Validate CloudWatch max memory
		if cloudwatchMaxMemory < 64 {
			return fmt.Errorf("cloudwatch max memory must be at least 64 MB, got %d", cloudwatchMaxMemory)
		}
		if cloudwatchMaxMemory > 4096 {
			return fmt.Errorf("cloudwatch max memory too large (max 4096 MB), got %d", cloudwatchMaxMemory)
		}
	}

	// Validate health port
	if healthPort < 1 || healthPort > 65535 {
		return fmt.Errorf("health port must be between 1 and 65535, got %d", healthPort)
	}

	// Console mode validation - if console mode is enabled, Insights config is optional
	if consoleMode {
		slog.Info("Console mode enabled - events will be printed to console instead of sent to Insights")
	}

	return nil
}

// getInsightsToken retrieves the Insights token from environment variables
func getInsightsToken(consoleMode bool) string {
	if consoleMode {
		// In console mode, token is not required
		return ""
	}

	token := strings.TrimSpace(os.Getenv("FAIRWINDS_TOKEN"))
	if token == "" {
		slog.Error("FAIRWINDS_TOKEN environment variable not set")
		os.Exit(1)
	}

	// Basic token validation
	if len(token) < 10 {
		slog.Error("FAIRWINDS_TOKEN is too short (minimum 10 characters)")
		os.Exit(1)
	}

	return token
}

func main() {
	var (
		logLevel               = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		insightsHost           = flag.String("insights-host", "", "Fairwinds Insights hostname")
		organization           = flag.String("organization", "", "Fairwinds organization name")
		cluster                = flag.String("cluster", "", "Cluster name")
		logSource              = flag.String("log-source", "local", "Log source type (local, cloudwatch)")
		auditLogPath           = flag.String("audit-log-path", "", "Path to Kubernetes audit log file (optional)")
		cloudwatchLogGroup     = flag.String("cloudwatch-log-group", "", "CloudWatch log group name (e.g., /aws/eks/production-eks/cluster)")
		cloudwatchRegion       = flag.String("cloudwatch-region", "", "AWS region for CloudWatch logs")
		cloudwatchFilter       = flag.String("cloudwatch-filter-pattern", "", "CloudWatch filter pattern for log events")
		cloudwatchBatchSize    = flag.Int("cloudwatch-batch-size", 100, "Number of log events to process in each batch")
		cloudwatchPollInterval = flag.String("cloudwatch-poll-interval", "30s", "Interval between CloudWatch log polls")
		cloudwatchMaxMemory    = flag.Int("cloudwatch-max-memory", 512, "Maximum memory usage in MB for CloudWatch processing")
		eventBufferSize        = flag.Int("event-buffer-size", 1000, "Size of the event processing buffer")
		httpTimeoutSeconds     = flag.Int("http-timeout-seconds", 30, "HTTP client timeout in seconds")
		rateLimitPerMinute     = flag.Int("rate-limit-per-minute", 60, "Maximum API calls per minute")
		consoleMode            = flag.Bool("console-mode", false, "Print events to console instead of sending to Insights")
		healthPort             = flag.Int("health-port", 8080, "Port for health check endpoints")
		shutdownTimeout        = flag.Duration("shutdown-timeout", 30*time.Second, "Graceful shutdown timeout")
	)
	flag.Parse()

	// Validate all configuration parameters
	if err := validateConfiguration(*logLevel, *insightsHost, *organization, *cluster, *auditLogPath, *logSource, *cloudwatchLogGroup, *cloudwatchRegion, *cloudwatchFilter, *eventBufferSize, *httpTimeoutSeconds, *rateLimitPerMinute, *cloudwatchBatchSize, *cloudwatchMaxMemory, *healthPort, *consoleMode); err != nil {
		slog.Error("Configuration validation failed", "error", err)
		os.Exit(1)
	}

	// Set up slog level
	var level slog.Level
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		slog.Error("Invalid log level", "level", *logLevel)
		os.Exit(1)
	}

	// Create a new logger with the specified level
	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	slog.Info("Starting Kubernetes Event Watcher")
	slog.Info("Configuration",
		"log_level", *logLevel,
		"log_source", *logSource,
		"console_mode", *consoleMode)

	// Log CloudWatch configuration if enabled
	if *logSource == "cloudwatch" {
		slog.Info("CloudWatch configuration",
			"log_group", *cloudwatchLogGroup,
			"region", *cloudwatchRegion,
			"filter_pattern", *cloudwatchFilter,
			"batch_size", *cloudwatchBatchSize,
			"max_memory_mb", *cloudwatchMaxMemory)
	}

	// Create Insights configuration
	insightsConfig := models.InsightsConfig{
		Hostname:     *insightsHost,
		Organization: *organization,
		Cluster:      *cluster,
		Token:        getInsightsToken(*consoleMode),
	}

	// Validate Insights configuration if provided
	if insightsConfig.Hostname != "" {
		if insightsConfig.Organization == "" || insightsConfig.Cluster == "" {
			slog.Error("If insights-host is provided, organization and cluster must also be provided")
			os.Exit(1)
		}
		slog.Info("Insights API configuration enabled",
			"hostname", insightsConfig.Hostname,
			"organization", insightsConfig.Organization,
			"cluster", insightsConfig.Cluster)
	} else if !*consoleMode {
		slog.Info("Insights API configuration not provided - running in local mode only")
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create CloudWatch configuration if needed
	var cloudwatchConfig *models.CloudWatchConfig
	if *logSource == "cloudwatch" {
		cloudwatchConfig = &models.CloudWatchConfig{
			LogGroupName:  *cloudwatchLogGroup,
			Region:        *cloudwatchRegion,
			FilterPattern: *cloudwatchFilter,
			BatchSize:     *cloudwatchBatchSize,
			PollInterval:  *cloudwatchPollInterval,
			MaxMemoryMB:   *cloudwatchMaxMemory,
		}
	}

	// Create watcher with configuration
	kubeWatcher, err := watcher.NewWatcher(insightsConfig, *logSource, *auditLogPath, cloudwatchConfig, *eventBufferSize, *httpTimeoutSeconds, *rateLimitPerMinute, *consoleMode)
	if err != nil {
		slog.Error("Failed to create watcher", "error", err)
		os.Exit(1)
	}

	// Log audit log configuration
	if *auditLogPath != "" {
		slog.Info("Audit log monitoring enabled", "audit_log_path", *auditLogPath)
	} else {
		slog.Info("Audit log monitoring disabled")
	}

	// Create health check server
	healthServer := health.NewServer(*healthPort, "1.0.0")

	// Register watcher health checker
	watcherChecker := health.NewWatcherChecker(kubeWatcher)
	healthServer.RegisterChecker(watcherChecker)

	// Start health check server
	if err := healthServer.Start(); err != nil {
		slog.Error("Failed to start health check server", "error", err)
		os.Exit(1)
	}

	// Start watcher
	if err := kubeWatcher.Start(ctx); err != nil {
		slog.Error("Failed to start watcher", "error", err)
		os.Exit(1)
	}

	// Wait for interrupt signal to gracefully shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Block until signal is received
	sig := <-sigChan
	slog.Info("Received signal, shutting down gracefully", "signal", sig)

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), *shutdownTimeout)
	defer shutdownCancel()

	// Stop watcher
	slog.Info("Stopping Kubernetes watcher")
	kubeWatcher.Stop()

	// Stop health check server
	slog.Info("Stopping health check server")
	if err := healthServer.Stop(shutdownCtx); err != nil {
		slog.Error("Failed to stop health check server gracefully", "error", err)
	}

	slog.Info("Kubernetes Event Watcher stopped")
}

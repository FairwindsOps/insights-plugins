package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/watcher"
)

// validateConfiguration validates all command-line parameters
func validateConfiguration(logLevel, insightsHost, organization, cluster, auditLogPath, logSource, cloudwatchLogGroup, cloudwatchRegion, cloudwatchFilter string, eventBufferSize, httpTimeoutSeconds, rateLimitPerMinute, cloudwatchBatchSize, cloudwatchMaxMemory int, consoleMode bool) error {
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
		if len(organization) > 100 {
			return fmt.Errorf("organization name too long (max 100 characters)")
		}
	}

	// Validate cluster name if provided
	if cluster != "" {
		if strings.TrimSpace(cluster) == "" {
			return fmt.Errorf("cluster name cannot be empty or whitespace only")
		}
		if len(cluster) > 100 {
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

	// Console mode validation - if console mode is enabled, Insights config is optional
	if consoleMode {
		logrus.Info("Console mode enabled - events will be printed to console instead of sent to Insights")
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
		logrus.Fatal("FAIRWINDS_TOKEN environment variable not set")
	}

	// Basic token validation
	if len(token) < 10 {
		logrus.Fatal("FAIRWINDS_TOKEN is too short (minimum 10 characters)")
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
	)
	flag.Parse()

	// Validate all configuration parameters
	if err := validateConfiguration(*logLevel, *insightsHost, *organization, *cluster, *auditLogPath, *logSource, *cloudwatchLogGroup, *cloudwatchRegion, *cloudwatchFilter, *eventBufferSize, *httpTimeoutSeconds, *rateLimitPerMinute, *cloudwatchBatchSize, *cloudwatchMaxMemory, *consoleMode); err != nil {
		logrus.WithError(err).Fatal("Configuration validation failed")
	}

	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		logrus.WithError(err).Fatal("Invalid log level")
	}
	logrus.SetLevel(level)

	logrus.Info("Starting Kubernetes Event Watcher")
	logrus.WithFields(logrus.Fields{
		"log_level":    *logLevel,
		"log_source":   *logSource,
		"console_mode": *consoleMode,
	}).Info("Configuration")

	// Log CloudWatch configuration if enabled
	if *logSource == "cloudwatch" {
		logrus.WithFields(logrus.Fields{
			"log_group":      *cloudwatchLogGroup,
			"region":         *cloudwatchRegion,
			"filter_pattern": *cloudwatchFilter,
			"batch_size":     *cloudwatchBatchSize,
			"max_memory_mb":  *cloudwatchMaxMemory,
		}).Info("CloudWatch configuration")
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
			logrus.Fatal("If insights-host is provided, organization and cluster must also be provided")
		}
		logrus.WithFields(logrus.Fields{
			"hostname":     insightsConfig.Hostname,
			"organization": insightsConfig.Organization,
			"cluster":      insightsConfig.Cluster,
		}).Info("Insights API configuration enabled")
	} else if !*consoleMode {
		logrus.Info("Insights API configuration not provided - running in local mode only")
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
		logrus.WithError(err).Fatal("Failed to create watcher")
	}

	// Log audit log configuration
	if *auditLogPath != "" {
		logrus.WithField("audit_log_path", *auditLogPath).Info("Audit log monitoring enabled")
	} else {
		logrus.Info("Audit log monitoring disabled")
	}

	// Start watcher
	if err := kubeWatcher.Start(ctx); err != nil {
		logrus.WithError(err).Fatal("Failed to start watcher")
	}

	// Wait for interrupt signal to gracefully shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Block until signal is received
	sig := <-sigChan
	logrus.WithField("signal", sig).Info("Received signal, shutting down gracefully")

	// Stop watcher
	kubeWatcher.Stop()

	logrus.Info("Kubernetes Event Watcher stopped")
}

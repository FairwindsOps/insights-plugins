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
func validateConfiguration(logLevel, insightsHost, organization, cluster, insightsToken, auditLogPath string, eventBufferSize, httpTimeoutSeconds, rateLimitPerMinute int) error {
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

	// Validate insights token if provided
	if insightsToken != "" {
		if strings.TrimSpace(insightsToken) == "" {
			return fmt.Errorf("insights token cannot be empty or whitespace only")
		}
		if len(insightsToken) < 10 {
			return fmt.Errorf("insights token too short (minimum 10 characters)")
		}
	}

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

	return nil
}

func main() {
	var (
		logLevel           = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		insightsHost       = flag.String("insights-host", "", "Fairwinds Insights hostname")
		organization       = flag.String("organization", "", "Fairwinds organization name")
		cluster            = flag.String("cluster", "", "Cluster name")
		insightsToken      = flag.String("insights-token", "", "Fairwinds Insights API token")
		auditLogPath       = flag.String("audit-log-path", "", "Path to Kubernetes audit log file (optional)")
		eventBufferSize    = flag.Int("event-buffer-size", 1000, "Size of the event processing buffer")
		httpTimeoutSeconds = flag.Int("http-timeout-seconds", 30, "HTTP client timeout in seconds")
		rateLimitPerMinute = flag.Int("rate-limit-per-minute", 60, "Maximum API calls per minute")
	)
	flag.Parse()

	// Validate all configuration parameters
	if err := validateConfiguration(*logLevel, *insightsHost, *organization, *cluster, *insightsToken, *auditLogPath, *eventBufferSize, *httpTimeoutSeconds, *rateLimitPerMinute); err != nil {
		logrus.WithError(err).Fatal("Configuration validation failed")
	}

	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		logrus.WithError(err).Fatal("Invalid log level")
	}
	logrus.SetLevel(level)

	logrus.Info("Starting Kubernetes Event Watcher")
	logrus.WithFields(logrus.Fields{
		"log_level": *logLevel,
	}).Info("Configuration")

	// Create Insights configuration
	insightsConfig := models.InsightsConfig{
		Hostname:     *insightsHost,
		Organization: *organization,
		Cluster:      *cluster,
		Token:        *insightsToken,
	}

	// Validate Insights configuration if provided
	if insightsConfig.Hostname != "" {
		if insightsConfig.Organization == "" || insightsConfig.Cluster == "" || insightsConfig.Token == "" {
			logrus.Fatal("If insights-host is provided, organization, cluster, and insights-token must also be provided")
		}
		logrus.WithFields(logrus.Fields{
			"hostname":     insightsConfig.Hostname,
			"organization": insightsConfig.Organization,
			"cluster":      insightsConfig.Cluster,
		}).Info("Insights API configuration enabled")
	} else {
		logrus.Info("Insights API configuration not provided - running in local mode only")
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create watcher with configuration
	kubeWatcher, err := watcher.NewWatcher(insightsConfig, *auditLogPath, *eventBufferSize, *httpTimeoutSeconds, *rateLimitPerMinute)
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

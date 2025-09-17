package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/watcher"
)

func main() {
	var (
		logLevel      = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		insightsHost  = flag.String("insights-host", "", "Fairwinds Insights hostname")
		organization  = flag.String("organization", "", "Fairwinds organization name")
		cluster       = flag.String("cluster", "", "Cluster name")
		insightsToken = flag.String("insights-token", "", "Fairwinds Insights API token")
	)
	flag.Parse()

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

	// Create watcher
	kubeWatcher, err := watcher.NewWatcher(insightsConfig)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create watcher")
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

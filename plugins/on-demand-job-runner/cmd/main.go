package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/FairwindsOps/insights-plugins/on-demand-job-runner/pkg/insights"
	"github.com/FairwindsOps/insights-plugins/on-demand-job-runner/pkg/k8s"
	"github.com/FairwindsOps/insights-plugins/on-demand-job-runner/pkg/ondemandjobs"
	"github.com/spf13/viper"
)

func loadConfig() (*ondemandjobs.Config, error) {
	// viper order of precedence:
	// 	1. Environment variables (e.g., ORGANIZATION, CLUSTER, TOKEN, HOST)
	// 	2. Config file (config.yaml)
	// 	3. Default values (if set in code)

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	viper.SetDefault("organization", "")
	viper.SetDefault("cluster", "")
	viper.SetDefault("token", "")
	viper.SetDefault("host", "https://insights.fairwinds.com")
	viper.SetDefault("maxConcurrentJobs", 10)
	viper.SetDefault("devMode", false)
	viper.SetDefault("pollInterval", "15s")

	err := viper.ReadInConfig()
	if err != nil {
		slog.Info("No config file found, using environment variables and defaults", "error", err)
	}

	requiredKeys := []string{"organization", "cluster", "token", "host"}
	for _, key := range requiredKeys {
		if !viper.IsSet(key) || viper.GetString(key) == "" {
			return nil, fmt.Errorf("required config key %q not set or empty", key)
		}
	}

	var c ondemandjobs.Config

	err = viper.Unmarshal(&c)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	return &c, nil
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	viper.WithLogger(logger)

	config, err := loadConfig()
	if err != nil {
		slog.Error("error loading configuration", "error", err)
		os.Exit(1)
	}

	slog.Info("Configuration loaded successfully", "organization", config.Organization, "cluster", config.Cluster, "host", config.Host, "token", strings.Repeat("*", len(config.Token)))

	insightsClient := insights.NewClient(config.Host, config.Token, config.Organization, config.Cluster, config.DevMode)

	clientset, err := k8s.GetClientSet()
	if err != nil {
		slog.Error("failed to create Kubernetes client", "error", err)
		os.Exit(1)
	}

	pollInterval, err := time.ParseDuration(config.PollInterval)
	if err != nil {
		slog.Error("error parsing poll interval", "error", err)
		os.Exit(1)
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for range ticker.C {
		err := ondemandjobs.FetchAndProcessOnDemandJobs(insightsClient, clientset, config.MaxConcurrentJobs)
		if err != nil {
			slog.Error("error processing on-demand jobs", "error", err)
			continue
		}
	}
}

package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/viper"
)

// Config represents the configuration for the Kyverno policy sync
type Config struct {
	// Insights configuration
	Host         string `mapstructure:"host"`
	Token        string `mapstructure:"token"`
	Organization string `mapstructure:"organization"`
	Cluster      string `mapstructure:"cluster"`
	DevMode      bool   `mapstructure:"devMode"`

	// Sync configuration
	DryRun           bool `mapstructure:"dryRun"`
	ValidatePolicies bool `mapstructure:"validatePolicies"`

	// Logging
	LogLevel string `mapstructure:"logLevel"`
}

// LoadConfig loads configuration from environment variables and config files
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/kyverno-policy-sync")

	// Set default values
	viper.SetDefault("devMode", false)
	viper.SetDefault("dryRun", false)
	viper.SetDefault("validatePolicies", true)
	viper.SetDefault("logLevel", "info")

	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found, continue with environment variables
	}

	// Bind environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("KYVERNO_SYNC")

	// Map environment variables to config keys
	viper.BindEnv("host", "FAIRWINDS_INSIGHTS_HOST")
	viper.BindEnv("token", "FAIRWINDS_TOKEN")
	viper.BindEnv("organization", "FAIRWINDS_ORG")
	viper.BindEnv("cluster", "FAIRWINDS_CLUSTER")
	viper.BindEnv("devMode", "FAIRWINDS_DEV_MODE")
	viper.BindEnv("dryRun", "DRY_RUN")
	viper.BindEnv("validatePolicies", "VALIDATE_POLICIES")
	viper.BindEnv("logLevel", "LOG_LEVEL")

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate required fields
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Set log level
	setLogLevel(config.LogLevel)

	return &config, nil
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	if config.Host == "" {
		return fmt.Errorf("host is required")
	}
	if config.Token == "" {
		return fmt.Errorf("token is required")
	}
	if config.Organization == "" {
		return fmt.Errorf("organization is required")
	}
	if config.Cluster == "" {
		return fmt.Errorf("cluster is required")
	}
	return nil
}

// setLogLevel sets the log level
func setLogLevel(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

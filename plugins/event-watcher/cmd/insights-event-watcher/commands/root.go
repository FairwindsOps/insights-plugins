package commands

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	// Global flags
	eventBufferSize    int
	httpTimeoutSeconds int
	rateLimitPerMinute int
	consoleMode        bool
	verbose            bool
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "insights-event-watcher",
	Short: "Monitor Kubernetes policy violations from audit logs and CloudWatch",
	Long: `Insights Event Watcher monitors Kubernetes policy violations from various sources:

- Local audit log files (for kind/local clusters)
- CloudWatch logs (for EKS clusters)

The watcher processes policy violations and sends them to Fairwinds Insights API.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Setup logging
		setupLogging()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags
	RootCmd.PersistentFlags().IntVar(&eventBufferSize, "buffer-size", 1000, "Event buffer size")
	RootCmd.PersistentFlags().IntVar(&httpTimeoutSeconds, "http-timeout", 30, "HTTP timeout in seconds")
	RootCmd.PersistentFlags().IntVar(&rateLimitPerMinute, "rate-limit", 60, "Rate limit per minute")
	RootCmd.PersistentFlags().BoolVar(&consoleMode, "console", false, "Enable console mode (print events to stdout)")
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
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

	return token
}

func setupLogging() {
	var level slog.Level
	if verbose {
		level = slog.LevelDebug
	} else {
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

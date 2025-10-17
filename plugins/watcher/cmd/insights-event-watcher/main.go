package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/cmd/insights-event-watcher/commands"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/health"
	"github.com/spf13/cobra"
)

func main() {
	// Add health check server command
	commands.RootCmd.AddCommand(healthCmd)

	// Execute the root command
	commands.Execute()
}

// healthCmd represents the health command
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Start health check server",
	Long: `Start a health check server that provides HTTP endpoints for monitoring.

This command starts an HTTP server that provides health check endpoints:
- /healthz - Kubernetes liveness probe
- /readyz - Kubernetes readiness probe  
- /health - General health status

The server runs in the background and can be used for Kubernetes health checks.`,
	RunE: runHealthServer,
}

var (
	healthPort    int
	healthAddress string
)

func init() {
	healthCmd.Flags().IntVar(&healthPort, "port", 8080, "Health check server port")
	healthCmd.Flags().StringVar(&healthAddress, "address", "0.0.0.0", "Health check server address")
}

func runHealthServer(cmd *cobra.Command, args []string) error {
	slog.Info("Starting health check server",
		"address", healthAddress,
		"port", healthPort)

	// Create health server
	server := health.NewServer(healthPort, "1.0.0")

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		slog.Info("Received shutdown signal, stopping health server...")
		cancel()
	}()

	// Start server in goroutine
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			slog.Error("Health server error", "error", err)
		}
	}()

	slog.Info("Health check server started successfully",
		"endpoints", []string{"/healthz", "/readyz", "/health"})

	// Wait for shutdown signal
	<-ctx.Done()

	// Stop server
	server.Stop(ctx)
	slog.Info("Health check server stopped")

	return nil
}

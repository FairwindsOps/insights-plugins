package health

import (
	"context"
	"fmt"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/metrics"
)

// MetricsProvider defines the interface for getting metrics
type MetricsProvider interface {
	GetMetrics() *metrics.Metrics
}

// WatcherChecker implements HealthChecker for the Kubernetes watcher
type WatcherChecker struct {
	watcher     MetricsProvider
	name        string
	minUptime   time.Duration
}

// NewWatcherChecker creates a new watcher health checker
func NewWatcherChecker(w MetricsProvider) *WatcherChecker {
	return &WatcherChecker{
		watcher:   w,
		name:      "kubernetes-watcher",
		minUptime: 5 * time.Second,
	}
}

// NewWatcherCheckerWithMinUptime creates a new watcher health checker with custom minimum uptime
func NewWatcherCheckerWithMinUptime(w MetricsProvider, minUptime time.Duration) *WatcherChecker {
	return &WatcherChecker{
		watcher:   w,
		name:      "kubernetes-watcher",
		minUptime: minUptime,
	}
}

// CheckHealth checks the health of the Kubernetes watcher
func (w *WatcherChecker) CheckHealth(ctx context.Context) error {
	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	// Check if the watcher is running by checking its metrics
	metrics := w.watcher.GetMetrics()
	if metrics == nil {
		return fmt.Errorf("watcher metrics not available")
	}

	// Check if the watcher has been running for a reasonable amount of time
	uptime := metrics.GetUptime()
	if uptime < w.minUptime {
		// If the watcher just started, it might not be fully ready yet
		return fmt.Errorf("watcher still starting up (uptime: %v)", uptime)
	}

	// Check if there are any critical issues
	processed, dropped := metrics.GetTotalEvents()
	
	// If we have a high drop rate, consider it unhealthy
	if processed > 0 {
		dropRate := float64(dropped) / float64(processed+dropped) * 100
		if dropRate > 50 {
			return fmt.Errorf("high event drop rate: %.2f%% (%d dropped out of %d total)", 
				dropRate, dropped, processed+dropped)
		}
	}

	// Check channel utilization
	utilization := metrics.GetChannelUtilization()
	if utilization > 90 {
		return fmt.Errorf("high channel utilization: %.2f%%", utilization)
	}

	return nil
}

// GetName returns the name of this health checker
func (w *WatcherChecker) GetName() string {
	return w.name
}

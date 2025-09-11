package main

import (
	"github.com/sirupsen/logrus"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/handlers"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
)

// CustomSecurityHandler is an example of how to create a custom event handler
type CustomSecurityHandler struct {
	insightsConfig models.InsightsConfig
}

// NewCustomSecurityHandler creates a new custom security handler
func NewCustomSecurityHandler(config models.InsightsConfig) *CustomSecurityHandler {
	return &CustomSecurityHandler{
		insightsConfig: config,
	}
}

// Handle processes the security-related event
func (h *CustomSecurityHandler) Handle(watchedEvent *event.WatchedEvent) error {
	logrus.WithFields(logrus.Fields{
		"event_type":    watchedEvent.EventType,
		"resource_type": watchedEvent.ResourceType,
		"namespace":     watchedEvent.Namespace,
		"name":          watchedEvent.Name,
	}).Info("Processing security event")

	// Add your custom logic here:
	// - Send to security monitoring system
	// - Trigger alerts
	// - Update security dashboards
	// - etc.

	return nil
}

// Example of how to register a custom handler
func registerCustomHandlers(factory *handlers.EventHandlerFactory, config models.InsightsConfig) {
	// Option 1: Register with specific name for event-based selection
	factory.Register("custom-security", NewCustomSecurityHandler(config))

	// Option 2: Register with resource type naming convention
	// For a CustomResource type, register as "customresource-handler"
	// factory.Register("customresource-handler", NewCustomResourceHandler(config))

	logrus.Info("Custom handlers registered")
}

// Example usage in main function:
func main() {
	// Create Insights configuration
	config := models.InsightsConfig{
		Hostname:     "https://insights.fairwinds.com",
		Organization: "my-org",
		Cluster:      "production",
		Token:        "your-token",
	}

	// Create factory
	factory := handlers.NewEventHandlerFactory(config)

	// Register custom handlers
	registerCustomHandlers(factory, config)

	// The factory will now automatically use your custom handler
	// when it encounters security-related events
}

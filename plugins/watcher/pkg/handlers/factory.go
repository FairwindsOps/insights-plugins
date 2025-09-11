package handlers

import (
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
)

// EventHandler interface for processing events
type EventHandler interface {
	Handle(watchedEvent *event.WatchedEvent) error
}

// EventHandlerFactory creates event handlers based on event characteristics
type EventHandlerFactory struct {
	insightsConfig models.InsightsConfig
	handlers       map[string]EventHandler
}

// NewEventHandlerFactory creates a new factory with registered handlers
func NewEventHandlerFactory(insightsConfig models.InsightsConfig) *EventHandlerFactory {
	factory := &EventHandlerFactory{
		insightsConfig: insightsConfig,
		handlers:       make(map[string]EventHandler),
	}

	// Register default handlers
	factory.registerDefaultHandlers()

	return factory
}

// registerDefaultHandlers registers all default event handlers
func (f *EventHandlerFactory) registerDefaultHandlers() {
	// PolicyViolation handler for Kubernetes events
	f.Register("policy-violation", NewPolicyViolationHandler(f.insightsConfig))

	// Resource-specific handlers using naming convention
	f.Register("policyreport-handler", NewKyvernoPolicyReportHandler(f.insightsConfig))
	f.Register("clusterpolicyreport-handler", NewKyvernoClusterPolicyReportHandler(f.insightsConfig))
	f.Register("policy-handler", NewKyvernoPolicyHandler(f.insightsConfig))
	f.Register("clusterpolicy-handler", NewKyvernoClusterPolicyHandler(f.insightsConfig))

	// Generic resource handler
	f.Register("generic-resource", NewGenericResourceHandler(f.insightsConfig))
}

// Register adds a new handler to the factory
func (f *EventHandlerFactory) Register(name string, handler EventHandler) {
	f.handlers[name] = handler
	logrus.WithField("handler_name", name).Debug("Registered event handler")
}

// GetHandler returns the appropriate handler for an event
func (f *EventHandlerFactory) GetHandler(watchedEvent *event.WatchedEvent) EventHandler {
	// Determine the handler name based on event characteristics
	handlerName := f.getHandlerName(watchedEvent)

	// Return the registered handler
	if handler, exists := f.handlers[handlerName]; exists {
		return handler
	}

	return nil
}

func (f *EventHandlerFactory) getHandlerName(watchedEvent *event.WatchedEvent) string {
	// Check for PolicyViolation events first (most specific)
	if watchedEvent.ResourceType == "events" {
		if reason, ok := watchedEvent.Data["reason"].(string); ok {
			if reason == "PolicyViolation" {
				return "policy-violation"
			}
		}
	}

	// For resource-specific handlers, use a simple naming convention:
	// ResourceType "PolicyReport" → handler name "policyreport-handler"
	resourceType := strings.ToLower(watchedEvent.ResourceType)
	handlerName := resourceType + "-handler"

	// Check if we have a specific handler for this resource type
	if _, exists := f.handlers[handlerName]; exists {
		return handlerName
	}

	// Default to generic handler
	return "generic-resource"
}

// ProcessEvent processes an event using the appropriate handler
func (f *EventHandlerFactory) ProcessEvent(watchedEvent *event.WatchedEvent) error {
	handler := f.GetHandler(watchedEvent)
	if handler == nil {
		logrus.WithFields(logrus.Fields{
			"event_type":    watchedEvent.EventType,
			"resource_type": watchedEvent.ResourceType,
			"namespace":     watchedEvent.Namespace,
			"name":          watchedEvent.Name,
		}).Debug("No handler found for event")
		return nil
	}

	return handler.Handle(watchedEvent)
}

func (f *EventHandlerFactory) GetHandlerCount() int {
	return len(f.handlers)
}

func (f *EventHandlerFactory) GetHandlerNames() []string {
	names := make([]string, 0, len(f.handlers))
	for name := range f.handlers {
		names = append(names, name)
	}
	return names
}

package handlers

import (
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

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
	kubeClient     kubernetes.Interface
	dynamicClient  dynamic.Interface
	handlers       map[string]EventHandler
}

// NewEventHandlerFactory creates a new factory with registered handlers
func NewEventHandlerFactory(insightsConfig models.InsightsConfig, kubeClient kubernetes.Interface, dynamicClient dynamic.Interface) *EventHandlerFactory {
	factory := &EventHandlerFactory{
		insightsConfig: insightsConfig,
		kubeClient:     kubeClient,
		dynamicClient:  dynamicClient,
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

	// ClusterPolicy Duplicator handler for ClusterPolicy resources
	f.Register("clusterpolicy-duplicator", NewClusterPolicyDuplicatorHandler(f.insightsConfig, f.dynamicClient))
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
	// Check for PolicyViolation and VAPViolation events first (most specific)
	if watchedEvent.ResourceType == "events" {
		if reason, ok := watchedEvent.Data["reason"].(string); ok {
			if reason == "PolicyViolation" || reason == "VAPViolation" {
				return "policy-violation"
			}
		}
	}

	// Check for ClusterPolicy resources
	if watchedEvent.ResourceType == "ClusterPolicy" {
		return "clusterpolicy-duplicator"
	}

	return ""
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

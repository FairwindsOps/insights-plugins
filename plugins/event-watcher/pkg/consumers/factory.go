package consumers

import (
	"log/slog"
	"strings"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/utils"
)

// EventHandler interface for processing events
type EventHandler interface {
	Handle(watchedEvent *models.WatchedEvent) error
}

// EventHandlerFactory creates event handlers based on event characteristics
type EventHandlerFactory struct {
	insightsConfig     models.InsightsConfig
	kubeClient         kubernetes.Interface
	dynamicClient      dynamic.Interface
	handlers           map[string]EventHandler
	httpTimeoutSeconds int
	rateLimitPerMinute int
}

// NewEventHandlerFactory creates a new factory with registered handlers
func NewEventHandlerFactory(insightsConfig models.InsightsConfig, kubeClient kubernetes.Interface, dynamicClient dynamic.Interface, httpTimeoutSeconds, rateLimitPerMinute int, consoleMode bool) *EventHandlerFactory {
	factory := &EventHandlerFactory{
		insightsConfig:     insightsConfig,
		kubeClient:         kubeClient,
		dynamicClient:      dynamicClient,
		handlers:           make(map[string]EventHandler),
		httpTimeoutSeconds: httpTimeoutSeconds,
		rateLimitPerMinute: rateLimitPerMinute,
	}

	// Register default handlers
	factory.registerDefaultHandlers(consoleMode)

	return factory
}

// registerDefaultHandlers registers all default event handlers
func (f *EventHandlerFactory) registerDefaultHandlers(consoleMode bool) {
	if consoleMode {
		// Console handler for printing events to console
		f.Register(utils.KyvernoPolicyViolationPrefix, NewConsoleHandler(f.insightsConfig))
		f.Register(utils.ValidatingPolicyViolationPrefix, NewConsoleHandler(f.insightsConfig))
		f.Register(utils.ValidatingAdmissionPolicyViolationPrefix, NewConsoleHandler(f.insightsConfig))
	} else {
		// PolicyViolation handler for Kubernetes events (sends to Insights)
		f.Register(utils.KyvernoPolicyViolationPrefix, NewPolicyViolationHandler(f.insightsConfig, f.httpTimeoutSeconds, f.rateLimitPerMinute))
		f.Register(utils.ValidatingPolicyViolationPrefix, NewValidatingPolicyViolationHandler(f.insightsConfig, f.httpTimeoutSeconds, f.rateLimitPerMinute))
		f.Register(utils.ValidatingAdmissionPolicyViolationPrefix, NewValidatingAdmissionPolicyViolationHandler(f.insightsConfig, f.httpTimeoutSeconds, f.rateLimitPerMinute))
		f.Register(utils.AuditOnlyAllowedValidatingAdmissionPolicyPrefix, NewAuditOnlyAllowedValidatingAdmissionPolicyHandler(f.insightsConfig, f.httpTimeoutSeconds, f.rateLimitPerMinute))
	}
}

// Register adds a new handler to the factory
func (f *EventHandlerFactory) Register(name string, handler EventHandler) {
	f.handlers[name] = handler
	slog.Debug("Registered event handler", "handler_name", name)
}

// GetHandler returns the appropriate handler for an event
func (f *EventHandlerFactory) GetHandler(watchedEvent *models.WatchedEvent) EventHandler {
	slog.Debug("Getting handler for event", "event_type", watchedEvent.EventType, "resource_type", watchedEvent.ResourceType, "namespace", watchedEvent.Namespace, "name", watchedEvent.Name)
	// Determine the handler name based on event characteristics
	handlerName := f.getHandlerName(watchedEvent)
	slog.Debug("Handler name", "handler_name", handlerName)
	// Return the registered handler
	if handler, exists := f.handlers[handlerName]; exists {
		slog.Debug("Found handler", "handler_name", handlerName)
		return handler
	}
	slog.Debug("No handler found", "handler_name", handlerName)
	return nil
}

func (f *EventHandlerFactory) getHandlerName(watchedEvent *models.WatchedEvent) string {
	// Check for PolicyViolation events first (most specific)
	slog.Debug("Getting handler name for event", "name", watchedEvent.Name)

	if strings.HasPrefix(watchedEvent.Name, utils.KyvernoPolicyViolationPrefix) {
		slog.Debug("Found kyverno policy violation event", "name", watchedEvent.Name)
		return utils.KyvernoPolicyViolationPrefix
	}
	if strings.HasPrefix(watchedEvent.Name, utils.ValidatingPolicyViolationPrefix) {
		slog.Debug("Found validating policy violation event", "name", watchedEvent.Name)
		return utils.ValidatingPolicyViolationPrefix
	}
	if strings.HasPrefix(watchedEvent.Name, utils.ValidatingAdmissionPolicyViolationPrefix) {
		slog.Debug("Found validating admission policy violation event", "name", watchedEvent.Name)
		return utils.ValidatingAdmissionPolicyViolationPrefix
	}
	if strings.HasPrefix(watchedEvent.Name, utils.AuditOnlyAllowedValidatingAdmissionPolicyPrefix) {
		slog.Debug("Found audit only allowed validating admission policy event", "name", watchedEvent.Name)
		return utils.AuditOnlyAllowedValidatingAdmissionPolicyPrefix
	}
	slog.Debug("No handler found for event", "name", watchedEvent.Name)
	return ""
}

// ProcessEvent processes an event using the appropriate handler
func (f *EventHandlerFactory) ProcessEvent(watchedEvent *models.WatchedEvent) error {
	slog.Debug("Processing event", "event_type", watchedEvent.EventType, "resource_type", watchedEvent.ResourceType, "namespace", watchedEvent.Namespace, "name", watchedEvent.Name)
	handler := f.GetHandler(watchedEvent)
	if handler == nil {
		slog.Debug("No handler found for event",
			"event_type", watchedEvent.EventType,
			"resource_type", watchedEvent.ResourceType,
			"namespace", watchedEvent.Namespace,
			"name", watchedEvent.Name)
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

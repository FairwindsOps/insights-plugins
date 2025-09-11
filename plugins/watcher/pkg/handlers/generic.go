package handlers

import (
	"github.com/sirupsen/logrus"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
)

// GenericResourceHandler handles any resource that doesn't have a specific handler
type GenericResourceHandler struct {
	insightsConfig models.InsightsConfig
}

// NewGenericResourceHandler creates a new generic resource handler
func NewGenericResourceHandler(config models.InsightsConfig) *GenericResourceHandler {
	return &GenericResourceHandler{
		insightsConfig: config,
	}
}

func (h *GenericResourceHandler) Handle(watchedEvent *event.WatchedEvent) error {
	logrus.WithFields(logrus.Fields{
		"event_type":    watchedEvent.EventType,
		"resource_type": watchedEvent.ResourceType,
		"namespace":     watchedEvent.Namespace,
		"name":          watchedEvent.Name,
		"uid":           watchedEvent.UID,
	}).Debug("Processing generic resource event")

	// Future: Could send all events to Insights API with a generic endpoint
	// return h.sendToInsights(watchedEvent)

	return nil
}

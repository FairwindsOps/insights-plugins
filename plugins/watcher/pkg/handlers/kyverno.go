package handlers

import (
	"github.com/sirupsen/logrus"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
)

// KyvernoPolicyReportHandler handles PolicyReport events
type KyvernoPolicyReportHandler struct {
	insightsConfig models.InsightsConfig
}

// NewKyvernoPolicyReportHandler creates a new PolicyReport handler
func NewKyvernoPolicyReportHandler(config models.InsightsConfig) *KyvernoPolicyReportHandler {
	return &KyvernoPolicyReportHandler{
		insightsConfig: config,
	}
}

func (h *KyvernoPolicyReportHandler) Handle(watchedEvent *event.WatchedEvent) error {
	logrus.WithFields(logrus.Fields{
		"event_type":    watchedEvent.EventType,
		"resource_type": watchedEvent.ResourceType,
		"namespace":     watchedEvent.Namespace,
		"name":          watchedEvent.Name,
	}).Info("Processing PolicyReport event")

	// Extract violations from the policy report
	if results, ok := watchedEvent.Data["results"].([]interface{}); ok {
		violations := 0
		for _, result := range results {
			if resultMap, ok := result.(map[string]interface{}); ok {
				if resultVal, ok := resultMap["result"].(string); ok {
					if resultVal == "fail" || resultVal == "warn" {
						violations++
					}
				}
			}
		}

		logrus.WithFields(logrus.Fields{
			"resource_type": watchedEvent.ResourceType,
			"name":          watchedEvent.Name,
			"namespace":     watchedEvent.Namespace,
			"violations":    violations,
		}).Info("PolicyReport processed")
	}

	return nil
}

// KyvernoClusterPolicyReportHandler handles ClusterPolicyReport events
type KyvernoClusterPolicyReportHandler struct {
	insightsConfig models.InsightsConfig
}

// NewKyvernoClusterPolicyReportHandler creates a new ClusterPolicyReport handler
func NewKyvernoClusterPolicyReportHandler(config models.InsightsConfig) *KyvernoClusterPolicyReportHandler {
	return &KyvernoClusterPolicyReportHandler{
		insightsConfig: config,
	}
}

func (h *KyvernoClusterPolicyReportHandler) Handle(watchedEvent *event.WatchedEvent) error {
	logrus.WithFields(logrus.Fields{
		"event_type":    watchedEvent.EventType,
		"resource_type": watchedEvent.ResourceType,
		"name":          watchedEvent.Name,
	}).Info("Processing ClusterPolicyReport event")

	// Extract violations from the cluster policy report
	if results, ok := watchedEvent.Data["results"].([]interface{}); ok {
		violations := 0
		for _, result := range results {
			if resultMap, ok := result.(map[string]interface{}); ok {
				if resultVal, ok := resultMap["result"].(string); ok {
					if resultVal == "fail" || resultVal == "warn" {
						violations++
					}
				}
			}
		}

		logrus.WithFields(logrus.Fields{
			"resource_type": watchedEvent.ResourceType,
			"name":          watchedEvent.Name,
			"violations":    violations,
		}).Info("ClusterPolicyReport processed")
	}

	return nil
}

// KyvernoPolicyHandler handles Policy events
type KyvernoPolicyHandler struct {
	insightsConfig models.InsightsConfig
}

// NewKyvernoPolicyHandler creates a new Policy handler
func NewKyvernoPolicyHandler(config models.InsightsConfig) *KyvernoPolicyHandler {
	return &KyvernoPolicyHandler{
		insightsConfig: config,
	}
}

func (h *KyvernoPolicyHandler) Handle(watchedEvent *event.WatchedEvent) error {
	logrus.WithFields(logrus.Fields{
		"event_type":    watchedEvent.EventType,
		"resource_type": watchedEvent.ResourceType,
		"namespace":     watchedEvent.Namespace,
		"name":          watchedEvent.Name,
	}).Info("Processing Policy event")

	// Log policy changes
	logrus.WithFields(logrus.Fields{
		"resource_type": watchedEvent.ResourceType,
		"name":          watchedEvent.Name,
		"namespace":     watchedEvent.Namespace,
		"event_type":    watchedEvent.EventType,
	}).Info("Policy event processed")

	return nil
}

// KyvernoClusterPolicyHandler handles ClusterPolicy events
type KyvernoClusterPolicyHandler struct {
	insightsConfig models.InsightsConfig
}

// NewKyvernoClusterPolicyHandler creates a new ClusterPolicy handler
func NewKyvernoClusterPolicyHandler(config models.InsightsConfig) *KyvernoClusterPolicyHandler {
	return &KyvernoClusterPolicyHandler{
		insightsConfig: config,
	}
}

func (h *KyvernoClusterPolicyHandler) Handle(watchedEvent *event.WatchedEvent) error {
	logrus.WithFields(logrus.Fields{
		"event_type":    watchedEvent.EventType,
		"resource_type": watchedEvent.ResourceType,
		"name":          watchedEvent.Name,
	}).Info("Processing ClusterPolicy event")

	// Log cluster policy changes
	logrus.WithFields(logrus.Fields{
		"resource_type": watchedEvent.ResourceType,
		"name":          watchedEvent.Name,
		"event_type":    watchedEvent.EventType,
	}).Info("ClusterPolicy event processed")

	return nil
}

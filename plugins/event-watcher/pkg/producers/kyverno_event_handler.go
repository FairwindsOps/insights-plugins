package producers

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/allegro/bigcache/v3"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/client"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
)

const (
	EventVersion                        = 1
	KyvernoPolicyViolationFieldSelector = "reason=PolicyViolation"
)

var alreadyProcessedKyvernoPolicyViolationIDs *bigcache.BigCache

func init() {
	var err error
	config := bigcache.DefaultConfig(60 * time.Minute)
	config.HardMaxCacheSize = 256 // 512MB
	alreadyProcessedKyvernoPolicyViolationIDs, err = bigcache.New(context.Background(), config)
	if err != nil {
		slog.Error("Failed to create bigcache", "error", err)
	}
	slog.Info("Bigcache created", "size", alreadyProcessedKyvernoPolicyViolationIDs.Len(), "hard_max_cache_size", config.HardMaxCacheSize)
}

type KubernetesEventHandler struct {
	eventChannel chan *models.WatchedEvent
	kubeClient   *client.Client
	pollInterval string
	stopCh       chan struct{}
}

// NewKubernetesEventHandler creates a new KubernetesEventHandler
func NewKubernetesEventHandler(insightsConfig models.InsightsConfig, kubeClient *client.Client, pollInterval string, eventChannel chan *models.WatchedEvent) *KubernetesEventHandler {
	return &KubernetesEventHandler{
		eventChannel: eventChannel,
		kubeClient:   kubeClient,
		pollInterval: pollInterval,
	}
}

// Start begins processing CloudWatch logs
func (h *KubernetesEventHandler) Start(ctx context.Context) error {
	slog.Info("Starting Kyverno Kubernetes event processing")

	// Parse poll interval
	pollInterval, err := time.ParseDuration(h.pollInterval)
	if err != nil {
		return fmt.Errorf("invalid poll interval '%s': %w", pollInterval, err)
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Kyverno Kubernetes event processing context cancelled")
			return ctx.Err()
		case <-ticker.C:
			if err := h.processKyvernoKubernetesEvents(ctx); err != nil {
				slog.Error("Failed to process kubernetesevents: ", "error", err)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

// Stop stops the CloudWatch handler
func (h *KubernetesEventHandler) Stop() {
	if h != nil && h.stopCh != nil {
		close(h.stopCh)
	}
}

/*
	Example Kyverno Kubernetes event

Name:             abc-require-labels.187486b992718f50
Namespace:        default
Labels:           <none>
Annotations:      <none>
Action:           Resource Passed
API Version:      v1
Event Time:       2025-11-03T14:51:25.792692Z
First Timestamp:  <nil>
Involved Object:

	API Version:   kyverno.io/v1
	Kind:          ClusterPolicy
	Name:          abc-require-labels
	UID:           abf48c44-6021-47d1-b0c9-42405aee09af

Kind:            Event
Last Timestamp:  <nil>
Message:         Deployment default/james1-deployment: [check-for-labels] fail; validation error: The label `abcapp.kubernetes.io/name` is required. rule check-for-labels failed at path /metadata/labels/abcapp.kubernetes.io/name/
Metadata:

	Creation Timestamp:  2025-11-03T14:51:25Z
	Resource Version:    8967
	UID:                 731e469c-7301-4423-9553-8a21091185e8

Reason:                PolicyViolation
Related:

	API Version:        apps/v1
	Kind:               Deployment
	Name:               james1-deployment
	Namespace:          default
	UID:                41a2ae97-2c0a-488e-8eae-d3cda56bc5fe

Reporting Component:  kyverno-admission
Reporting Instance:   kyverno-admission-kyverno-admission-controller-bfb99d565-84vjd
Source:
Type:    Warning
Events:  <none>
*/
func (h *KubernetesEventHandler) processKyvernoKubernetesEvents(ctx context.Context) error {
	fieldSelector, err := fields.ParseSelector(KyvernoPolicyViolationFieldSelector)
	if err != nil {
		return fmt.Errorf("failed to parse field selector: %w", err)
	}
	slog.Debug("Field selector: ", "fieldSelector", fieldSelector.String())
	options := metav1.ListOptions{
		FieldSelector: fieldSelector.String(),
	}
	events, err := h.kubeClient.KubeInterface.CoreV1().Events("").List(ctx, options)
	slog.Info("Processing Kyverno Kubernetes events", "events", len(events.Items))
	if err != nil {
		return fmt.Errorf("failed to list latest kubernetes events: %w", err)
	}
	for _, event := range events.Items {
		if !utils.IsAuditOnlyClusterPolicyViolation(event) {
			slog.Debug("Skipping non-audit only cluster policy violation event", "event", event)
			continue
		}
		key := fmt.Sprintf("%s-%s-%s-%s", event.InvolvedObject.Namespace, event.InvolvedObject.Name, event.InvolvedObject.Kind, event.ObjectMeta.UID)
		if value, err := alreadyProcessedKyvernoPolicyViolationIDs.Get(key); err == nil && value != nil {
			slog.Debug("Kyverno policy violation ID already processed, skipping", "kyverno_policy_violation_id", key)
			continue
		}
		err = alreadyProcessedKyvernoPolicyViolationIDs.Set(key, []byte("true"))
		if err != nil {
			slog.Warn("Failed to set kyverno policy violation ID in bigcache", "error", err, "kyverno_policy_violation_id", event.ObjectMeta.UID)
		}
		event := &models.WatchedEvent{
			EventVersion: EventVersion,
			Timestamp:    event.EventTime.Unix(),
			EventTime:    event.EventTime.UTC().Format(time.RFC3339),
			EventType:    models.EventTypeAdded,
			Kind:         event.Related.Kind,
			Namespace:    event.Related.Namespace,
			Name:         fmt.Sprintf("%s-%s-%s-%s", utils.AuditOnlyClusterPolicyViolationPrefix, event.Related.Kind, event.Related.Name, event.ObjectMeta.UID),
			UID:          string(event.Related.UID),
			Data: map[string]interface{}{
				"message":           event.Message,
				"policyName":        event.InvolvedObject.Name,
				"annotations":       event.ObjectMeta.Annotations,
				"labels":            event.ObjectMeta.Labels,
				"creationTimestamp": event.ObjectMeta.CreationTimestamp.Format(time.RFC3339),
				"resourceVersion":   event.ObjectMeta.ResourceVersion,
				"uid":               event.ObjectMeta.UID,
				"reason":            event.Reason,
			},
			Metadata: map[string]interface{}{
				"policyName":        event.InvolvedObject.Name,
				"annotations":       event.ObjectMeta.Annotations,
				"labels":            event.ObjectMeta.Labels,
				"creationTimestamp": event.ObjectMeta.CreationTimestamp.Format(time.RFC3339),
				"resourceVersion":   event.ObjectMeta.ResourceVersion,
				"uid":               event.ObjectMeta.UID,
				"reason":            event.Reason,
				"message":           event.Message,
				"related":           event.Related,
				"reportingInstance": event.ReportingInstance,
				"source":            event.Source,
				"type":              event.Type,
			},
			EventSource: "kubernetes_events",
			Success:     false,
			Blocked:     false,
		}
		slog.Info("Sending audit only cluster policy violation event", "event", event)
		h.eventChannel <- event
	}
	return nil
}

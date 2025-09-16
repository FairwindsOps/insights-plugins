package vap_monitor

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	admissionv1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
)

// VAPMonitor monitors ValidatingAdmissionPolicy resources and generates synthetic events
type VAPMonitor struct {
	client         kubernetes.Interface
	eventChannel   chan *event.WatchedEvent
	insightsConfig models.InsightsConfig
	stopCh         chan struct{}
}

// NewVAPMonitor creates a new VAP monitor
func NewVAPMonitor(client kubernetes.Interface, insightsConfig models.InsightsConfig) *VAPMonitor {
	return &VAPMonitor{
		client:         client,
		eventChannel:   make(chan *event.WatchedEvent, 100),
		insightsConfig: insightsConfig,
		stopCh:         make(chan struct{}),
	}
}

// Start begins monitoring VAP resources
func (m *VAPMonitor) Start(ctx context.Context) error {
	logrus.Info("Starting VAP Monitor")

	// Watch for changes to VAPs and their bindings
	go m.watchVAPs(ctx)
	go m.watchVAPBindings(ctx)
	go m.processEvents(ctx)

	return nil
}

// Stop stops the monitor
func (m *VAPMonitor) Stop() {
	close(m.stopCh)
}

// watchVAPs monitors ValidatingAdmissionPolicy resources
func (m *VAPMonitor) watchVAPs(ctx context.Context) {
	watcher, err := m.client.AdmissionregistrationV1beta1().ValidatingAdmissionPolicies().Watch(ctx, metav1.ListOptions{})
	if err != nil {
		logrus.WithError(err).Error("Failed to watch ValidatingAdmissionPolicies")
		return
	}
	defer watcher.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ctx.Done():
			return
		case event := <-watcher.ResultChan():
			if err := m.handleVAPEvent(event); err != nil {
				logrus.WithError(err).Error("Failed to handle VAP event")
			}
		}
	}
}

// watchVAPBindings monitors ValidatingAdmissionPolicyBinding resources
func (m *VAPMonitor) watchVAPBindings(ctx context.Context) {
	watcher, err := m.client.AdmissionregistrationV1beta1().ValidatingAdmissionPolicyBindings().Watch(ctx, metav1.ListOptions{})
	if err != nil {
		logrus.WithError(err).Error("Failed to watch ValidatingAdmissionPolicyBindings")
		return
	}
	defer watcher.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ctx.Done():
			return
		case event := <-watcher.ResultChan():
			if err := m.handleVAPBindingEvent(event); err != nil {
				logrus.WithError(err).Error("Failed to handle VAP binding event")
			}
		}
	}
}

// handleVAPEvent processes ValidatingAdmissionPolicy events
func (m *VAPMonitor) handleVAPEvent(kubeEvent watch.Event) error {
	vap, ok := kubeEvent.Object.(*admissionv1.ValidatingAdmissionPolicy)
	if !ok {
		return fmt.Errorf("failed to convert to ValidatingAdmissionPolicy")
	}

	var eventType event.EventType
	switch kubeEvent.Type {
	case watch.Added:
		eventType = event.EventTypeAdded
	case watch.Modified:
		eventType = event.EventTypeModified
	case watch.Deleted:
		eventType = event.EventTypeDeleted
	default:
		return fmt.Errorf("unknown event type: %s", kubeEvent.Type)
	}

	// Create a synthetic event for VAP changes
	watchedEvent, err := m.createVAPEvent(eventType, vap)
	if err != nil {
		return fmt.Errorf("failed to create VAP event: %w", err)
	}

	select {
	case m.eventChannel <- watchedEvent:
		// Event successfully queued
	default:
		logrus.Warn("VAP event channel full, dropping event")
	}

	return nil
}

// handleVAPBindingEvent processes ValidatingAdmissionPolicyBinding events
func (m *VAPMonitor) handleVAPBindingEvent(kubeEvent watch.Event) error {
	binding, ok := kubeEvent.Object.(*admissionv1.ValidatingAdmissionPolicyBinding)
	if !ok {
		return fmt.Errorf("failed to convert to ValidatingAdmissionPolicyBinding")
	}

	var eventType event.EventType
	switch kubeEvent.Type {
	case watch.Added:
		eventType = event.EventTypeAdded
	case watch.Modified:
		eventType = event.EventTypeModified
	case watch.Deleted:
		eventType = event.EventTypeDeleted
	default:
		return fmt.Errorf("unknown event type: %s", kubeEvent.Type)
	}

	// Create a synthetic event for VAP binding changes
	watchedEvent, err := m.createVAPBindingEvent(eventType, binding)
	if err != nil {
		return fmt.Errorf("failed to create VAP binding event: %w", err)
	}

	select {
	case m.eventChannel <- watchedEvent:
		// Event successfully queued
	default:
		logrus.Warn("VAP binding event channel full, dropping event")
	}

	return nil
}

// createVAPEvent creates a synthetic event for VAP changes
func (m *VAPMonitor) createVAPEvent(eventType event.EventType, vap *admissionv1.ValidatingAdmissionPolicy) (*event.WatchedEvent, error) {
	// Create metadata
	metadata := map[string]interface{}{
		"name":      vap.Name,
		"namespace": "", // VAPs are cluster-scoped
		"uid":       string(vap.UID),
	}

	// Create event data
	eventData := map[string]interface{}{
		"kind":       "ValidatingAdmissionPolicy",
		"apiVersion": "admissionregistration.k8s.io/v1beta1",
		"metadata":   metadata,
		"spec":       vap.Spec,
		"status":     vap.Status,
	}

	watchedEvent := &event.WatchedEvent{
		EventVersion: event.EventVersion,
		Timestamp:    time.Now().Unix(),
		EventType:    eventType,
		ResourceType: "ValidatingAdmissionPolicy",
		Namespace:    "",
		Name:         vap.Name,
		UID:          string(vap.UID),
		Data:         eventData,
		Metadata:     metadata,
	}

	return watchedEvent, nil
}

// createVAPBindingEvent creates a synthetic event for VAP binding changes
func (m *VAPMonitor) createVAPBindingEvent(eventType event.EventType, binding *admissionv1.ValidatingAdmissionPolicyBinding) (*event.WatchedEvent, error) {
	// Create metadata
	metadata := map[string]interface{}{
		"name":      binding.Name,
		"namespace": "", // VAP bindings are cluster-scoped
		"uid":       string(binding.UID),
	}

	// Create event data
	eventData := map[string]interface{}{
		"kind":       "ValidatingAdmissionPolicyBinding",
		"apiVersion": "admissionregistration.k8s.io/v1beta1",
		"metadata":   metadata,
		"spec":       binding.Spec,
	}

	watchedEvent := &event.WatchedEvent{
		EventVersion: event.EventVersion,
		Timestamp:    time.Now().Unix(),
		EventType:    eventType,
		ResourceType: "ValidatingAdmissionPolicyBinding",
		Namespace:    "",
		Name:         binding.Name,
		UID:          string(binding.UID),
		Data:         eventData,
		Metadata:     metadata,
	}

	return watchedEvent, nil
}

// processEvents processes VAP-related events
func (m *VAPMonitor) processEvents(ctx context.Context) {
	for {
		select {
		case <-m.stopCh:
			return
		case <-ctx.Done():
			return
		case watchedEvent := <-m.eventChannel:
			watchedEvent.LogEvent()

			// Log VAP-specific information
			logrus.WithFields(logrus.Fields{
				"event_type":    watchedEvent.EventType,
				"resource_type": watchedEvent.ResourceType,
				"name":          watchedEvent.Name,
				"vap_monitor":   true,
			}).Info("VAP resource change detected")
		}
	}
}

// GenerateSyntheticVAPViolationEvent creates a synthetic PolicyViolation event for a VAP violation
func (m *VAPMonitor) GenerateSyntheticVAPViolationEvent(policyName, resourceType, namespace, name, message string) error {
	eventName := fmt.Sprintf("%s.%x", name, time.Now().UnixNano())

	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      eventName,
			Namespace: namespace,
		},
		Reason:  "PolicyViolation",
		Message: fmt.Sprintf("ValidatingAdmissionPolicy '%s' denied request: %s", policyName, message),
		Source: corev1.EventSource{
			Component: "vap-monitor",
		},
		Type:           "Warning",
		Count:          1,
		FirstTimestamp: metav1.Time{Time: time.Now()},
		LastTimestamp:  metav1.Time{Time: time.Now()},
		InvolvedObject: corev1.ObjectReference{
			Kind:      resourceType,
			Name:      name,
			Namespace: namespace,
		},
	}

	// Create the event
	_, err := m.client.CoreV1().Events(namespace).Create(context.Background(), event, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create synthetic VAP violation event: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"event_name":    eventName,
		"policy_name":   policyName,
		"resource_type": resourceType,
		"namespace":     namespace,
		"name":          name,
		"message":       message,
	}).Info("Generated synthetic VAP violation event")

	return nil
}

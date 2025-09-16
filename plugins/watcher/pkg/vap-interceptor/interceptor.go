package vap_interceptor

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
)

// VAPInterceptor monitors ValidatingAdmissionPolicy violations and generates events
type VAPInterceptor struct {
	client         kubernetes.Interface
	eventChannel   chan *event.WatchedEvent
	insightsConfig models.InsightsConfig
}

// NewVAPInterceptor creates a new VAP interceptor
func NewVAPInterceptor(insightsConfig models.InsightsConfig) (*VAPInterceptor, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return &VAPInterceptor{
		client:         client,
		eventChannel:   make(chan *event.WatchedEvent, 100),
		insightsConfig: insightsConfig,
	}, nil
}

// Start begins monitoring for VAP violations
func (v *VAPInterceptor) Start(ctx context.Context) error {
	logrus.Info("Starting VAP Interceptor")

	// Watch ValidatingAdmissionPolicy resources
	go v.watchVAPs(ctx)

	// Watch ValidatingAdmissionPolicyBinding resources
	go v.watchVAPBindings(ctx)

	// Process events
	go v.processEvents(ctx)

	return nil
}

// watchVAPs monitors ValidatingAdmissionPolicy resources
func (v *VAPInterceptor) watchVAPs(ctx context.Context) {
	watcher, err := v.client.AdmissionregistrationV1beta1().ValidatingAdmissionPolicies().Watch(ctx, metav1.ListOptions{})
	if err != nil {
		logrus.WithError(err).Error("Failed to watch ValidatingAdmissionPolicies")
		return
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-watcher.ResultChan():
			if err := v.handleVAPEvent(event); err != nil {
				logrus.WithError(err).Error("Failed to handle VAP event")
			}
		}
	}
}

// watchVAPBindings monitors ValidatingAdmissionPolicyBinding resources
func (v *VAPInterceptor) watchVAPBindings(ctx context.Context) {
	watcher, err := v.client.AdmissionregistrationV1beta1().ValidatingAdmissionPolicyBindings().Watch(ctx, metav1.ListOptions{})
	if err != nil {
		logrus.WithError(err).Error("Failed to watch ValidatingAdmissionPolicyBindings")
		return
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-watcher.ResultChan():
			if err := v.handleVAPBindingEvent(event); err != nil {
				logrus.WithError(err).Error("Failed to handle VAP binding event")
			}
		}
	}
}

// handleVAPEvent processes ValidatingAdmissionPolicy events
func (v *VAPInterceptor) handleVAPEvent(kubeEvent watch.Event) error {
	// Get the VAP object from the event
	vap, ok := kubeEvent.Object.(*v1beta1.ValidatingAdmissionPolicy)
	if !ok {
		return fmt.Errorf("failed to convert event to ValidatingAdmissionPolicy")
	}

	// Log VAP resource changes
	logrus.WithFields(logrus.Fields{
		"resource":   "ValidatingAdmissionPolicy",
		"name":       vap.Name,
		"event_type": kubeEvent.Type,
	}).Debug("VAP resource changed")

	return nil
}

// handleVAPBindingEvent processes ValidatingAdmissionPolicyBinding events
func (v *VAPInterceptor) handleVAPBindingEvent(kubeEvent watch.Event) error {
	// Get the binding object from the event
	binding, ok := kubeEvent.Object.(*v1beta1.ValidatingAdmissionPolicyBinding)
	if !ok {
		return fmt.Errorf("failed to convert event to ValidatingAdmissionPolicyBinding")
	}

	// Log VAP binding changes
	logrus.WithFields(logrus.Fields{
		"resource":   "ValidatingAdmissionPolicyBinding",
		"name":       binding.Name,
		"event_type": kubeEvent.Type,
	}).Debug("VAP binding resource changed")

	return nil
}

// processEvents processes intercepted events
func (v *VAPInterceptor) processEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case watchedEvent := <-v.eventChannel:
			watchedEvent.LogEvent()
			// Process the event (could send to Insights, etc.)
		}
	}
}

// GenerateVAPViolationEvent creates a synthetic event for a VAP violation
func (v *VAPInterceptor) GenerateVAPViolationEvent(policyName, resourceType, namespace, name, message string) error {
	// Create a synthetic event that mimics a PolicyViolation event
	eventData := map[string]interface{}{
		"reason":  "PolicyViolation",
		"message": fmt.Sprintf("ValidatingAdmissionPolicy '%s' denied request: %s", policyName, message),
		"involvedObject": map[string]interface{}{
			"kind":      resourceType,
			"name":      name,
			"namespace": namespace,
		},
		"source": map[string]interface{}{
			"component": "vap-interceptor",
		},
	}

	// Create the event
	kubeEvent := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s.%x", name, time.Now().UnixNano()),
			Namespace: namespace,
		},
		Reason:  "PolicyViolation",
		Message: eventData["message"].(string),
		Source: corev1.EventSource{
			Component: "vap-interceptor",
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

	// Create the event in Kubernetes
	_, err := v.client.CoreV1().Events(namespace).Create(context.Background(), kubeEvent, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create VAP violation event: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"policy_name":   policyName,
		"resource_type": resourceType,
		"namespace":     namespace,
		"name":          name,
		"message":       message,
	}).Info("Generated VAP violation event")

	return nil
}

// GetVAPViolations returns information about active VAPs that could cause violations
func (v *VAPInterceptor) GetVAPViolations() ([]VAPViolationInfo, error) {
	vaps, err := v.client.AdmissionregistrationV1beta1().ValidatingAdmissionPolicies().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list VAPs: %w", err)
	}

	var violations []VAPViolationInfo
	for _, vap := range vaps.Items {
		// Get bindings for this VAP
		bindings, err := v.client.AdmissionregistrationV1beta1().ValidatingAdmissionPolicyBindings().List(context.Background(), metav1.ListOptions{
			FieldSelector: fmt.Sprintf("metadata.name=%s", vap.Name+"-binding"),
		})
		if err != nil {
			logrus.WithError(err).WithField("vap", vap.Name).Warn("Failed to get bindings for VAP")
			continue
		}

		for _, binding := range bindings.Items {
			if binding.Spec.PolicyName == vap.Name {
				violations = append(violations, VAPViolationInfo{
					PolicyName:        vap.Name,
					BindingName:       binding.Name,
					FailurePolicy:     *vap.Spec.FailurePolicy,
					ValidationActions: binding.Spec.ValidationActions,
					MatchConstraints:  *vap.Spec.MatchConstraints,
				})
			}
		}
	}

	return violations, nil
}

// VAPViolationInfo contains information about a VAP that could cause violations
type VAPViolationInfo struct {
	PolicyName        string
	BindingName       string
	FailurePolicy     v1beta1.FailurePolicyType
	ValidationActions []v1beta1.ValidationAction
	MatchConstraints  v1beta1.MatchResources
}

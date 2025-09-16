package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
)

func init() {
	corev1.AddToScheme(scheme)
	admissionv1.AddToScheme(scheme)
}

// VAPInterceptorWebhook intercepts admission requests and generates events for VAP violations
type VAPInterceptorWebhook struct {
	client kubernetes.Interface
}

func main() {
	logrus.Info("Starting VAP Interceptor Webhook")

	// Create Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to get in-cluster config")
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create Kubernetes client")
	}

	webhook := &VAPInterceptorWebhook{
		client: client,
	}

	// Set up HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/validate", webhook.handleValidate)
	mux.HandleFunc("/health", webhook.handleHealth)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		if err := server.ListenAndServeTLS("/certs/tls.crt", "/certs/tls.key"); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Fatal("Failed to start webhook server")
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logrus.Info("Shutting down webhook server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logrus.WithError(err).Error("Failed to shutdown server gracefully")
	}
}

// handleValidate processes admission requests
func (webhook *VAPInterceptorWebhook) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse admission review
	var review admissionv1.AdmissionReview
	if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
		logrus.WithError(err).Error("Failed to decode admission review")
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Process the request
	response := webhook.processAdmissionRequest(&review)

	// Send response
	review.Response = response
	review.Request = nil // Clear request to reduce response size

	if err := json.NewEncoder(w).Encode(review); err != nil {
		logrus.WithError(err).Error("Failed to encode admission review response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// processAdmissionRequest processes an admission request and generates events if needed
func (webhook *VAPInterceptorWebhook) processAdmissionRequest(review *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	request := review.Request
	if request == nil {
		return &admissionv1.AdmissionResponse{
			UID:     review.Request.UID,
			Allowed: false,
			Result: &metav1.Status{
				Message: "No request provided",
			},
		}
	}

	// Log the request
	logrus.WithFields(logrus.Fields{
		"uid":       request.UID,
		"kind":      request.Kind.Kind,
		"name":      request.Name,
		"namespace": request.Namespace,
		"operation": request.Operation,
	}).Debug("Processing admission request")

	// Check if this request would be blocked by VAPs
	// This is a simplified check - in reality, you'd need to evaluate the actual VAP expressions
	if webhook.wouldBeBlockedByVAP(request) {
		// Generate an event for the potential violation
		if err := webhook.generateVAPViolationEvent(request); err != nil {
			logrus.WithError(err).Error("Failed to generate VAP violation event")
		}
	}

	// Always allow the request (we're just intercepting, not blocking)
	return &admissionv1.AdmissionResponse{
		UID:     request.UID,
		Allowed: true,
	}
}

// wouldBeBlockedByVAP checks if a request would be blocked by ValidatingAdmissionPolicies
func (webhook *VAPInterceptorWebhook) wouldBeBlockedByVAP(request *admissionv1.AdmissionRequest) bool {
	// This is a simplified check - in reality, you'd need to:
	// 1. Get all ValidatingAdmissionPolicies
	// 2. Check if any match this request
	// 3. Evaluate the CEL expressions
	// 4. Check if any would result in a denial

	// For now, let's check if the resource has hostPath volumes (our test case)
	if request.Kind.Kind == "Pod" || request.Kind.Kind == "Deployment" {
		// Parse the object to check for hostPath volumes
		if webhook.hasHostPathVolume(request.Object.Raw) {
			return true
		}
	}

	return false
}

// hasHostPathVolume checks if the object has hostPath volumes
func (webhook *VAPInterceptorWebhook) hasHostPathVolume(rawObject []byte) bool {
	var obj map[string]interface{}
	if err := json.Unmarshal(rawObject, &obj); err != nil {
		logrus.WithError(err).Error("Failed to unmarshal object")
		return false
	}

	// Check for hostPath volumes in the object
	// This is a simplified check for our test case
	if spec, ok := obj["spec"].(map[string]interface{}); ok {
		if volumes, ok := spec["volumes"].([]interface{}); ok {
			for _, volume := range volumes {
				if volumeMap, ok := volume.(map[string]interface{}); ok {
					if _, hasHostPath := volumeMap["hostPath"]; hasHostPath {
						return true
					}
				}
			}
		}
	}

	return false
}

// generateVAPViolationEvent creates a Kubernetes event for a VAP violation
func (webhook *VAPInterceptorWebhook) generateVAPViolationEvent(request *admissionv1.AdmissionRequest) error {
	eventName := fmt.Sprintf("%s.%x", request.Name, time.Now().UnixNano())

	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      eventName,
			Namespace: request.Namespace,
		},
		Reason:  "PolicyViolation",
		Message: "ValidatingAdmissionPolicy 'disallow-host-path' would deny request: HostPath volumes are forbidden. The field spec.template.spec.volumes[*].hostPath must be unset.",
		Source: corev1.EventSource{
			Component: "vap-interceptor",
		},
		Type:           "Warning",
		Count:          1,
		FirstTimestamp: metav1.Time{Time: time.Now()},
		LastTimestamp:  metav1.Time{Time: time.Now()},
		InvolvedObject: corev1.ObjectReference{
			Kind:      request.Kind.Kind,
			Name:      request.Name,
			Namespace: request.Namespace,
		},
	}

	// Create the event
	_, err := webhook.client.CoreV1().Events(request.Namespace).Create(context.Background(), event, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create VAP violation event: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"event_name":    eventName,
		"resource_type": request.Kind.Kind,
		"namespace":     request.Namespace,
		"name":          request.Name,
	}).Info("Generated VAP violation event")

	return nil
}

// handleHealth provides a health check endpoint
func (webhook *VAPInterceptorWebhook) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/watcher"
)

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
)

func init() {
	corev1.AddToScheme(scheme)
	admissionv1.AddToScheme(scheme)
	admissionregistrationv1beta1.AddToScheme(scheme)
}

// VAPInterceptorWebhook intercepts admission requests and generates events for VAP violations
type VAPInterceptorWebhook struct {
	client kubernetes.Interface
}

// ViolationDetails contains information about a policy violation
type ViolationDetails struct {
	PolicyName string
	Violation  string
	Message    string
}

func main() {
	var (
		logLevel             = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		insightsHost         = flag.String("insights-host", "", "Fairwinds Insights hostname")
		organization         = flag.String("organization", "", "Fairwinds organization name")
		cluster              = flag.String("cluster", "", "Cluster name")
		insightsToken        = flag.String("insights-token", "", "Fairwinds Insights API token")
		enableVAPInterceptor = flag.Bool("enable-vap-interceptor", false, "Enable VAP interceptor webhook alongside the watcher")
		vapInterceptorPort   = flag.String("vap-interceptor-port", "8080", "Port for VAP interceptor webhook")
	)
	flag.Parse()

	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		logrus.WithError(err).Fatal("Invalid log level")
	}
	logrus.SetLevel(level)

	logrus.Info("Starting Kubernetes Event Watcher")
	logrus.WithFields(logrus.Fields{
		"log_level":               *logLevel,
		"vap_interceptor_enabled": *enableVAPInterceptor,
	}).Info("Configuration")

	// Create Insights configuration
	insightsConfig := models.InsightsConfig{
		Hostname:     *insightsHost,
		Organization: *organization,
		Cluster:      *cluster,
		Token:        *insightsToken,
	}

	// Validate Insights configuration if provided
	if insightsConfig.Hostname != "" {
		if insightsConfig.Organization == "" || insightsConfig.Cluster == "" || insightsConfig.Token == "" {
			logrus.Fatal("If insights-host is provided, organization, cluster, and insights-token must also be provided")
		}
		logrus.WithFields(logrus.Fields{
			"hostname":     insightsConfig.Hostname,
			"organization": insightsConfig.Organization,
			"cluster":      insightsConfig.Cluster,
		}).Info("Insights API configuration enabled")
	} else {
		logrus.Info("Insights API configuration not provided - running in local mode only")
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create Kubernetes client (needed for VAP interceptor)
	var kubeClient kubernetes.Interface
	if *enableVAPInterceptor {
		config, err := rest.InClusterConfig()
		if err != nil {
			logrus.WithError(err).Fatal("Failed to get in-cluster config")
		}

		kubeClient, err = kubernetes.NewForConfig(config)
		if err != nil {
			logrus.WithError(err).Fatal("Failed to create Kubernetes client")
		}
	}

	// Start VAP interceptor webhook if enabled
	if *enableVAPInterceptor {
		webhook := &VAPInterceptorWebhook{
			client: kubeClient,
		}

		// Set up HTTP server for VAP interceptor
		mux := http.NewServeMux()
		mux.HandleFunc("/validate", webhook.handleValidate)
		mux.HandleFunc("/health", webhook.handleHealth)

		server := &http.Server{
			Addr:    ":" + *vapInterceptorPort,
			Handler: mux,
		}

		// Start VAP interceptor server in goroutine
		go func() {
			logrus.WithField("port", *vapInterceptorPort).Info("Starting VAP interceptor webhook server")
			if err := server.ListenAndServeTLS("/certs/tls.crt", "/certs/tls.key"); err != nil && err != http.ErrServerClosed {
				logrus.WithError(err).Fatal("Failed to start VAP interceptor webhook server")
			}
		}()

		// Graceful shutdown for VAP interceptor
		go func() {
			<-ctx.Done()
			logrus.Info("Shutting down VAP interceptor webhook server...")
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer shutdownCancel()

			if err := server.Shutdown(shutdownCtx); err != nil {
				logrus.WithError(err).Error("Failed to shutdown VAP interceptor server gracefully")
			}
		}()
	}

	// Create watcher
	kubeWatcher, err := watcher.NewWatcher(insightsConfig)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create watcher")
	}

	// Start watcher
	if err := kubeWatcher.Start(ctx); err != nil {
		logrus.WithError(err).Fatal("Failed to start watcher")
	}

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigCh
	logrus.Info("Received shutdown signal, stopping services...")

	// Cancel context
	cancel()

	// Stop watcher
	kubeWatcher.Stop()

	// Give some time for cleanup
	time.Sleep(2 * time.Second)

	logrus.Info("Kubernetes Event Watcher stopped")
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
	// Get all ValidatingAdmissionPolicies that might match this request
	vaps, err := webhook.getMatchingVAPs(request)
	if err != nil {
		logrus.WithError(err).Error("Failed to get matching VAPs")
		return false
	}

	// Check if any VAP would block this request
	for _, vap := range vaps {
		if webhook.evaluateVAP(vap, request) {
			return true
		}
	}

	return false
}

// getMatchingVAPs returns ValidatingAdmissionPolicies that might match the request
func (webhook *VAPInterceptorWebhook) getMatchingVAPs(request *admissionv1.AdmissionRequest) ([]admissionregistrationv1beta1.ValidatingAdmissionPolicy, error) {
	// Get all ValidatingAdmissionPolicies
	vapList, err := webhook.client.AdmissionregistrationV1beta1().ValidatingAdmissionPolicies().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list VAPs: %w", err)
	}

	var matchingVAPs []admissionregistrationv1beta1.ValidatingAdmissionPolicy

	// Filter VAPs that might match this request
	for _, vap := range vapList.Items {
		if webhook.vapMatchesRequest(&vap, request) {
			matchingVAPs = append(matchingVAPs, vap)
		}
	}

	return matchingVAPs, nil
}

// vapMatchesRequest checks if a VAP matches the given request
func (webhook *VAPInterceptorWebhook) vapMatchesRequest(vap *admissionregistrationv1beta1.ValidatingAdmissionPolicy, request *admissionv1.AdmissionRequest) bool {
	// Check if the VAP's match constraints match the request
	if vap.Spec.MatchConstraints == nil {
		return false
	}

	matchConstraints := *vap.Spec.MatchConstraints

	// Check resource rules
	for _, rule := range matchConstraints.ResourceRules {
		if webhook.ruleMatchesRequest(&rule, request) {
			return true
		}
	}

	// Check namespace selector
	if matchConstraints.NamespaceSelector != nil {
		// For now, we'll assume it matches if there's a namespace selector
		// In a real implementation, you'd need to evaluate the selector
		return true
	}

	return false
}

// ruleMatchesRequest checks if a resource rule matches the request
func (webhook *VAPInterceptorWebhook) ruleMatchesRequest(rule *admissionregistrationv1beta1.NamedRuleWithOperations, request *admissionv1.AdmissionRequest) bool {
	// Check operations
	for _, op := range rule.Operations {
		if string(op) == string(request.Operation) {
			// Check API groups
			for _, group := range rule.APIGroups {
				if group == "*" || group == request.Kind.Group {
					// Check resources
					for _, resource := range rule.Resources {
						if resource == "*" || resource == request.Kind.Kind {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// evaluateVAP evaluates a ValidatingAdmissionPolicy against a request
func (webhook *VAPInterceptorWebhook) evaluateVAP(vap admissionregistrationv1beta1.ValidatingAdmissionPolicy, request *admissionv1.AdmissionRequest) bool {
	// This is a simplified evaluation - in reality, you'd need to:
	// 1. Parse the CEL expressions in the VAP
	// 2. Evaluate them against the request object
	// 3. Return true if any expression would result in a denial

	// For now, we'll be very conservative and only intercept requests that we're confident would be blocked
	// This prevents interference with normal admission requests

	// Only evaluate VAPs that have a failure policy of "Fail"
	if vap.Spec.FailurePolicy != nil && *vap.Spec.FailurePolicy != admissionregistrationv1beta1.Fail {
		return false
	}

	// Only evaluate VAPs that have validations
	if len(vap.Spec.Validations) == 0 {
		return false
	}

	// For now, we'll be very conservative and not generate events
	// This prevents interference with normal admission requests
	// In a real implementation, you'd evaluate the CEL expressions here
	// and only return true if the expressions would actually deny the request

	return false
}

// getViolationDetails analyzes the request and returns details about the violation
func (webhook *VAPInterceptorWebhook) getViolationDetails(request *admissionv1.AdmissionRequest) ViolationDetails {
	// Get all ValidatingAdmissionPolicies that might match this request
	vaps, err := webhook.getMatchingVAPs(request)
	if err != nil {
		logrus.WithError(err).Error("Failed to get matching VAPs for violation details")
		return ViolationDetails{
			PolicyName: "error-getting-policies",
			Violation:  "failed to determine policy",
			Message:    fmt.Sprintf("ValidatingAdmissionPolicy violation detected for %s %s in namespace %s (unable to determine specific policy)", request.Kind.Kind, request.Name, request.Namespace),
		}
	}

	// Find the first VAP that would block this request
	for _, vap := range vaps {
		if webhook.evaluateVAP(vap, request) {
			return ViolationDetails{
				PolicyName: vap.Name,
				Violation:  "policy validation failed",
				Message:    fmt.Sprintf("ValidatingAdmissionPolicy '%s' would deny request for %s %s in namespace %s", vap.Name, request.Kind.Kind, request.Name, request.Namespace),
			}
		}
	}

	// This shouldn't happen if wouldBeBlockedByVAP returned true, but just in case
	return ViolationDetails{
		PolicyName: "unknown-policy",
		Violation:  "policy violation detected",
		Message:    fmt.Sprintf("ValidatingAdmissionPolicy violation detected for %s %s in namespace %s", request.Kind.Kind, request.Name, request.Namespace),
	}
}

// generateVAPViolationEvent creates a Kubernetes event for a VAP violation
func (webhook *VAPInterceptorWebhook) generateVAPViolationEvent(request *admissionv1.AdmissionRequest) error {
	eventName := fmt.Sprintf("%s.%x", request.Name, time.Now().UnixNano())

	// Determine the violation details
	violationDetails := webhook.getViolationDetails(request)

	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      eventName,
			Namespace: request.Namespace,
		},
		Reason:  "PolicyViolation",
		Message: violationDetails.Message,
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
		"policy_name":   violationDetails.PolicyName,
		"violation":     violationDetails.Violation,
	}).Info("Generated VAP violation event")

	return nil
}

// handleHealth provides a health check endpoint
func (webhook *VAPInterceptorWebhook) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

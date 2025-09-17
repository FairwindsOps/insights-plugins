package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	k8sConfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

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

// VAPMutator handles mutating admission requests for ValidatingAdmissionPolicyBinding resources
type VAPMutator struct {
	client  kubernetes.Interface
	decoder *admission.Decoder
}

// VAPValidator handles validating admission requests
type VAPValidator struct {
	client  kubernetes.Interface
	decoder *admission.Decoder
}

// InjectDecoder injects the decoder
func (m *VAPMutator) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}

// InjectDecoder injects the decoder
func (v *VAPValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}

// Handle processes validating admission requests
func (v *VAPValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	// Log the request
	logrus.WithFields(logrus.Fields{
		"uid":       req.UID,
		"kind":      req.Kind.Kind,
		"name":      req.Name,
		"namespace": req.Namespace,
		"operation": req.Operation,
	}).Debug("Processing validating admission request")

	// For now, always allow requests (we're just intercepting, not blocking)
	return admission.Allowed("")
}

// Handle processes mutating admission requests for ValidatingAdmissionPolicyBinding resources
func (m *VAPMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	// Only process ValidatingAdmissionPolicyBinding resources
	if req.Kind.Kind != "ValidatingAdmissionPolicyBinding" {
		return admission.Allowed("")
	}

	logrus.WithFields(logrus.Fields{
		"resource":  req.Kind.Kind,
		"name":      req.Name,
		"namespace": req.Namespace,
		"operation": req.Operation,
	}).Info("Processing ValidatingAdmissionPolicyBinding for mutation")

	// Parse the ValidatingAdmissionPolicyBinding
	var binding admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding
	decoder := *m.decoder
	if err := decoder.Decode(req, &binding); err != nil {
		logrus.WithError(err).Error("Failed to decode ValidatingAdmissionPolicyBinding")
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Check if we need to add Audit action
	modified := m.addAuditToBinding(&binding)
	if !modified {
		// No changes needed, allow the request
		return admission.Allowed("")
	}

	// Create the patch to add Audit action
	patch, err := m.createAuditPatch(&binding)
	if err != nil {
		logrus.WithError(err).Error("Failed to create audit patch")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	logrus.WithFields(logrus.Fields{
		"binding_name": binding.Name,
		"policy_name":  binding.Spec.PolicyName,
		"patch_count":  len(patch),
	}).Info("Adding Audit action to ValidatingAdmissionPolicyBinding")

	return admission.Patched("", patch...)
}

// addAuditToBinding checks if Audit action should be added to the binding
func (m *VAPMutator) addAuditToBinding(binding *admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding) bool {
	// Check if validationActions contains only Deny
	actions := binding.Spec.ValidationActions
	if len(actions) != 1 || actions[0] != admissionregistrationv1beta1.Deny {
		// Not a Deny-only binding, no changes needed
		return false
	}

	// Check if Audit is already present
	for _, action := range actions {
		if action == admissionregistrationv1beta1.Audit {
			// Audit already present, no changes needed
			return false
		}
	}

	// Add Audit to the validationActions
	binding.Spec.ValidationActions = append(binding.Spec.ValidationActions, admissionregistrationv1beta1.Audit)
	return true
}

// createAuditPatch creates a JSON patch to add Audit action to the binding
func (m *VAPMutator) createAuditPatch(binding *admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding) ([]jsonpatch.Operation, error) {
	// Create a JSON patch to add Audit to validationActions
	patch := []jsonpatch.Operation{
		{
			Operation: "add",
			Path:      "/spec/validationActions/-",
			Value:     "Audit",
		},
	}

	return patch, nil
}

func main() {
	var (
		logLevel             = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		insightsHost         = flag.String("insights-host", "", "Fairwinds Insights hostname")
		organization         = flag.String("organization", "", "Fairwinds organization name")
		cluster              = flag.String("cluster", "", "Cluster name")
		insightsToken        = flag.String("insights-token", "", "Fairwinds Insights API token")
		enableVAPInterceptor = flag.Bool("enable-vap-interceptor", false, "Enable VAP interceptor webhook alongside the watcher")
		vapInterceptorPort   = flag.String("vap-interceptor-port", "8443", "Port for VAP interceptor webhook")
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

	// Start VAP interceptor webhook if enabled
	if *enableVAPInterceptor {
		// Get Kubernetes config
		k8sCfg := k8sConfig.GetConfigOrDie()
		clientset, err := kubernetes.NewForConfig(k8sCfg)
		if err != nil {
			logrus.WithError(err).Fatal("Failed to create Kubernetes client")
		}

		// Parse webhook port
		webhookPort := int64(8443)
		if *vapInterceptorPort != "" {
			webhookPort, err = strconv.ParseInt(*vapInterceptorPort, 10, 0)
			if err != nil {
				logrus.WithError(err).Fatal("Failed to parse webhook port")
			}
		}

		// Create controller-runtime manager with webhook server
		mgr, err := manager.New(k8sCfg, manager.Options{
			HealthProbeBindAddress: ":8081",
			WebhookServer: webhook.NewServer(webhook.Options{
				Port:     int(webhookPort),
				CertDir:  "/opt/cert",
				CertName: "tls.crt",
				KeyName:  "tls.key",
			}),
		})
		if err != nil {
			logrus.WithError(err).Fatal("Unable to set up overall controller manager")
		}

		// Add health checks
		err = mgr.AddReadyzCheck("readyz", healthz.Ping)
		if err != nil {
			logrus.WithError(err).Fatal("Unable to add readyz check")
		}
		err = mgr.AddHealthzCheck("healthz", healthz.Ping)
		if err != nil {
			logrus.WithError(err).Fatal("Unable to add healthz check")
		}

		// Check for certificate existence
		_, err = os.Stat("/opt/cert/tls.crt")
		if os.IsNotExist(err) {
			logrus.Warn("Certificate does not exist at /opt/cert/tls.crt - webhook will not start until certificate is available")
		}

		// Create webhook handlers
		mutator := &VAPMutator{client: clientset}
		validator := &VAPValidator{client: clientset}

		// Register webhook handlers
		mgr.GetWebhookServer().Register("/mutate", &webhook.Admission{Handler: mutator})
		mgr.GetWebhookServer().Register("/validate", &webhook.Admission{Handler: validator})

		// Start webhook server in goroutine
		go func() {
			logrus.WithField("port", webhookPort).Info("Starting VAP interceptor webhook server with TLS")
			if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
				logrus.WithError(err).Error("Error starting webhook manager")
			}
		}()

		// Start VAP event monitor
		go func() {
			logrus.Info("Starting VAP event monitor")
			webhook := &VAPInterceptorWebhook{client: clientset}
			webhook.startVAPEventMonitor(ctx)
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

// handleMutate handles mutating admission requests for ValidatingAdmissionPolicyBinding resources
func (webhook *VAPInterceptorWebhook) handleMutate(w http.ResponseWriter, r *http.Request) {
	// Read the admission review
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.WithError(err).Error("Failed to read request body")
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Parse the admission review
	var review admissionv1.AdmissionReview
	if err := json.Unmarshal(body, &review); err != nil {
		logrus.WithError(err).Error("Failed to parse admission review")
		http.Error(w, "Failed to parse admission review", http.StatusBadRequest)
		return
	}

	// Process the mutating admission request
	response := webhook.processMutatingAdmissionRequest(&review)

	// Send response
	review.Response = response
	review.Request = nil // Clear request to reduce response size

	if err := json.NewEncoder(w).Encode(review); err != nil {
		logrus.WithError(err).Error("Failed to encode admission review response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// processMutatingAdmissionRequest processes mutating admission requests for ValidatingAdmissionPolicyBinding resources
func (webhook *VAPInterceptorWebhook) processMutatingAdmissionRequest(review *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	request := review.Request
	if request == nil {
		return &admissionv1.AdmissionResponse{
			UID:     review.Request.UID,
			Allowed: false,
			Result: &metav1.Status{
				Message: "No admission request found",
			},
		}
	}

	// Only process ValidatingAdmissionPolicyBinding resources
	if request.Kind.Kind != "ValidatingAdmissionPolicyBinding" {
		return &admissionv1.AdmissionResponse{
			UID:     request.UID,
			Allowed: true,
		}
	}

	logrus.WithFields(logrus.Fields{
		"resource":  request.Kind.Kind,
		"name":      request.Name,
		"namespace": request.Namespace,
		"operation": request.Operation,
	}).Info("Processing ValidatingAdmissionPolicyBinding for mutation")

	// Parse the ValidatingAdmissionPolicyBinding
	var binding admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding
	if err := json.Unmarshal(request.Object.Raw, &binding); err != nil {
		logrus.WithError(err).Error("Failed to unmarshal ValidatingAdmissionPolicyBinding")
		return &admissionv1.AdmissionResponse{
			UID:     request.UID,
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("Failed to unmarshal ValidatingAdmissionPolicyBinding: %v", err),
			},
		}
	}

	// Check if we need to add Audit action
	modified := webhook.addAuditToBinding(&binding)
	if !modified {
		// No changes needed, allow the request
		return &admissionv1.AdmissionResponse{
			UID:     request.UID,
			Allowed: true,
		}
	}

	// Create the patch to add Audit action
	patch, err := webhook.createAuditPatch(&binding)
	if err != nil {
		logrus.WithError(err).Error("Failed to create audit patch")
		return &admissionv1.AdmissionResponse{
			UID:     request.UID,
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("Failed to create audit patch: %v", err),
			},
		}
	}

	logrus.WithFields(logrus.Fields{
		"binding_name": binding.Name,
		"policy_name":  binding.Spec.PolicyName,
		"patch":        string(patch),
	}).Info("Adding Audit action to ValidatingAdmissionPolicyBinding")

	return &admissionv1.AdmissionResponse{
		UID:     request.UID,
		Allowed: true,
		Patch:   patch,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

// addAuditToBinding checks if Audit action should be added to the binding
func (webhook *VAPInterceptorWebhook) addAuditToBinding(binding *admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding) bool {
	// Check if validationActions contains only Deny
	actions := binding.Spec.ValidationActions
	if len(actions) != 1 || actions[0] != admissionregistrationv1beta1.Deny {
		// Not a Deny-only binding, no changes needed
		return false
	}

	// Check if Audit is already present
	for _, action := range actions {
		if action == admissionregistrationv1beta1.Audit {
			// Audit already present, no changes needed
			return false
		}
	}

	// Add Audit to the validationActions
	binding.Spec.ValidationActions = append(binding.Spec.ValidationActions, admissionregistrationv1beta1.Audit)
	return true
}

// createAuditPatch creates a JSON patch to add Audit action to the binding
func (webhook *VAPInterceptorWebhook) createAuditPatch(binding *admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding) ([]byte, error) {
	// Create a JSON patch to add Audit to validationActions
	patch := []map[string]interface{}{
		{
			"op":    "add",
			"path":  "/spec/validationActions/-",
			"value": "Audit",
		},
	}

	return json.Marshal(patch)
}

// startVAPEventMonitor monitors for VAP-related events and generates synthetic events
func (webhook *VAPInterceptorWebhook) startVAPEventMonitor(ctx context.Context) {
	// Watch for events that contain VAP violation information
	eventWatcher, err := webhook.client.CoreV1().Events("").Watch(ctx, metav1.ListOptions{
		Watch: true,
	})
	if err != nil {
		logrus.WithError(err).Error("Failed to start event watcher for VAP monitoring")
		return
	}
	defer eventWatcher.Stop()

	logrus.Info("VAP event monitor started, watching for VAP violations")

	for {
		select {
		case <-ctx.Done():
			logrus.Info("VAP event monitor stopping")
			return
		case event := <-eventWatcher.ResultChan():
			if event.Type == "ADDED" || event.Type == "MODIFIED" {
				if kubeEvent, ok := event.Object.(*corev1.Event); ok {
					webhook.processVAPEvent(kubeEvent)
				}
			}
		}
	}
}

// processVAPEvent processes events to detect VAP violations
func (webhook *VAPInterceptorWebhook) processVAPEvent(event *corev1.Event) {
	// Check if this event is related to VAP violations
	if webhook.isVAPViolationEvent(event) {
		logrus.WithFields(logrus.Fields{
			"event_name":      event.Name,
			"event_namespace": event.Namespace,
			"reason":          event.Reason,
			"message":         event.Message,
		}).Info("Detected VAP violation event")

		// Generate synthetic event for Insights
		if err := webhook.generateSyntheticVAPEvent(event); err != nil {
			logrus.WithError(err).Error("Failed to generate synthetic VAP event")
		}
	}
}

// isVAPViolationEvent checks if an event represents a VAP violation
func (webhook *VAPInterceptorWebhook) isVAPViolationEvent(event *corev1.Event) bool {
	// Check for VAP-related keywords in the event message
	vapKeywords := []string{
		"ValidatingAdmissionPolicy",
		"denied request",
		"forbidden",
		"validation failed",
		"policy violation",
		"VAP Policy Violation",
	}

	message := strings.ToLower(event.Message)
	for _, keyword := range vapKeywords {
		if strings.Contains(message, strings.ToLower(keyword)) {
			return true
		}
	}

	// Check for VAP-related reasons
	vapReasons := []string{
		"FailedValidation",
		"PolicyViolation",
		"AdmissionError",
		"VAPViolation",
	}

	reason := strings.ToLower(event.Reason)
	for _, vapReason := range vapReasons {
		if strings.Contains(reason, strings.ToLower(vapReason)) {
			return true
		}
	}

	return false
}

// generateSyntheticVAPEvent generates a synthetic event for VAP violations
func (webhook *VAPInterceptorWebhook) generateSyntheticVAPEvent(event *corev1.Event) error {
	// Create a synthetic event that mimics a PolicyViolation event
	syntheticEvent := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("vap-violation-%s", event.Name),
			Namespace: event.Namespace,
		},
		InvolvedObject: event.InvolvedObject,
		Reason:         "VAPViolation",
		Message:        fmt.Sprintf("VAP Policy Violation: %s", event.Message),
		Source: corev1.EventSource{
			Component: "vap-interceptor",
			Host:      "vap-interceptor",
		},
		FirstTimestamp: event.FirstTimestamp,
		LastTimestamp:  event.LastTimestamp,
		Count:          event.Count,
		Type:           "Warning",
	}

	// Create the synthetic event in the cluster
	_, err := webhook.client.CoreV1().Events(event.Namespace).Create(context.Background(), syntheticEvent, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create synthetic VAP event: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"original_event":  event.Name,
		"synthetic_event": syntheticEvent.Name,
		"namespace":       event.Namespace,
	}).Info("Generated synthetic VAP violation event")

	return nil
}

// handleHealth provides a health check endpoint
func (webhook *VAPInterceptorWebhook) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

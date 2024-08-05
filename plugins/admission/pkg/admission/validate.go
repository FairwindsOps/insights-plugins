package admission

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/kube"
	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/opa"
	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/pluto"
	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/polaris"
)

// webhookFailurePolicy stores the states of a Kubernetes
// ValidatingWebhookConfiguration webhook failurePolicy.
type webhookFailurePolicy int

// webhookFailurePolicy* constants represent possible values for a Kubernetes
// ValidatingWebhookConfiguration webhook failurePolicy.
const (
	webhookFailurePolicyIgnore = iota
	webhookFailurePolicyFail   = iota
)

// Admission webhooks can optionally return warning messages that are returned to
// the requesting client in HTTP Warning headers with a warning code of 299
// Ref: https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#response
const httpStatusMiscPersistentWarning = 299

func (w webhookFailurePolicy) String() string {
	var result string
	switch w {
	case webhookFailurePolicyIgnore:
		result = "Ignore"
	case webhookFailurePolicyFail:
		result = "Fail"
	default:
		result = "unknown"
	}
	return result
}

// Validator is the entry point for the admission webhook.
type Validator struct {
	clientset            *kubernetes.Clientset
	iConfig              models.InsightsConfig
	decoder              *admission.Decoder
	config               *models.Configuration
	webhookFailurePolicy webhookFailurePolicy
}

func NewValidator(clientset *kubernetes.Clientset, iConfig models.InsightsConfig) *Validator {
	return &Validator{
		iConfig:   iConfig,
		clientset: clientset,
	}
}

// SetWebhookFailurePolicy parses a string into one of the
// webhookFailurePolicy* constants and sets the webhookFailurePolicy field of
// the Validator struct. SetWebhookFailurePolicy returns true if the string is
// parsed successfully.
func (v *Validator) SetWebhookFailurePolicy(s string) bool {
	switch strings.ToLower(s) {
	case "":
		v.webhookFailurePolicy = 0 // set empty string to the default iota
	case "ignore":
		v.webhookFailurePolicy = webhookFailurePolicyIgnore
	case "fail":
		v.webhookFailurePolicy = webhookFailurePolicyFail
	default:
		logrus.Infof("unknown webhook failure policy %q", s)
		return false
	}
	logrus.Infof("using webhook failure policy %q", v.webhookFailurePolicy)
	return true
}

// InjectDecoder injects the decoder.
func (v *Validator) InjectDecoder(d admission.Decoder) error {
	logrus.Info("Injecting decoder")
	v.decoder = &d
	return nil
}

// InjectConfig injects the config.
func (v *Validator) InjectConfig(c models.Configuration) error {
	logrus.Info("Injecting config")
	v.config = &c
	return nil
}

func (v *Validator) handleInternal(ctx context.Context, req admission.Request) (bool, []string, []string, error) {
	username := req.UserInfo.Username
	if lo.Contains(v.iConfig.IgnoreUsernames, username) {
		msg := fmt.Sprintf("Insights admission controller is ignoring service account %s.", username)
		return true, []string{msg}, nil, nil
	}
	rawBytes := req.Object.Raw
	if req.Operation == "DELETE" {
		rawBytes = req.OldObject.Raw // Object.Raw is empty for DELETEs
	}
	var decoded map[string]any
	err := json.Unmarshal(rawBytes, &decoded)
	if err != nil {
		logrus.Errorf("Error unmarshaling JSON")
		return false, nil, nil, err
	}
	if ownerReferences, ok := decoded["metadata"].(map[string]any)["ownerReferences"].([]any); ok && len(ownerReferences) > 0 {
		ownerReference := ownerReferences[0].(map[string]any)
		client := kube.GetKubeClient()
		controller, err := client.GetObject(ctx, req.Namespace, ownerReference["kind"].(string), ownerReference["apiVersion"].(string), ownerReference["name"].(string), client.DynamicInterface, client.RestMapper)
		if err != nil {
			logrus.Infof("error retrieving owner for object %s - running checks: %v", req.Name, err)
		} else {
			err = ValidateIfControllerMatches(decoded, controller.Object)
			if err != nil {
				logrus.Infof("object %s has an owner but the owner is invalid - running checks: %v", req.Name, err)
			} else {
				logrus.Infof("object %s has an owner and the owner is valid - skipping", req.Name)
				return true, nil, nil, nil
			}
		}
	} else {
		logrus.Infof("Object %s has no owner - running checks", req.Name)
	}
	var namespaceMetadata map[string]any
	if namespace, ok := decoded["metadata"].(map[string]any)["namespace"].(string); ok && namespace != "" {
		namespaceMetadata, err = getNamespaceMetadata(v.clientset, namespace)
		if err != nil {
			return false, nil, nil, err
		}
	}
	return processInputYAML(ctx, v.iConfig, *v.config, decoded, req, namespaceMetadata)
}

func getNamespaceMetadata(clientset *kubernetes.Clientset, namespace string) (map[string]any, error) {
	ns, err := clientset.CoreV1().Namespaces().Get(context.Background(), namespace, v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"kind":       ns.Kind,
		"apiVersion": ns.APIVersion,
		"metadata": map[string]any{
			"labels":      ns.Labels,
			"annotations": ns.Annotations,
		},
	}, nil
}

// Handle for Validator to run validation checks.
func (v *Validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	fairwindsInsightsIndicator := "[Fairwinds Insights]"
	blockedIndicator := "[Blocked]"
	logrus.Infof("Starting %s request for %s%s/%s %s in namespace %s", req.Operation, req.RequestKind.Group, req.RequestKind.Version, req.RequestKind.Kind, req.Name, req.Namespace)
	allowed, warnings, errors, err := v.handleInternal(ctx, req)
	if err != nil {
		logrus.Errorf("Error validating request: %v", err)
		if v.webhookFailurePolicy != webhookFailurePolicyIgnore {
			logrus.Infoln("Failing validation request due to errors, as failurePolicy is not set to ignore")
			return admission.Errored(http.StatusBadRequest, err)
		} else if v.webhookFailurePolicy == webhookFailurePolicyIgnore {
			allowed = true
			logrus.Warningf("allowing request despite errors, as webhook failurePolicy is set to %s", v.webhookFailurePolicy)
		}
	}
	response := admission.ValidationResponse(allowed, strings.Join(errors, ", "))
	if len(warnings) > 0 {
		response.Result.Code = httpStatusMiscPersistentWarning
		for _, warnString := range warnings {
			response.Warnings = append(response.Warnings, fmt.Sprintf("%s %s", fairwindsInsightsIndicator, warnString))
		}
	}
	if len(errors) > 0 {
		// add errors to warnings for increased readability in command-line
		for _, errString := range errors {
			response.Warnings = append(response.Warnings, fmt.Sprintf("%s %s %s", fairwindsInsightsIndicator, blockedIndicator, errString))
		}
	}
	logrus.Infof("%d warnings returned: %s", len(warnings), strings.Join(warnings, ", "))
	logrus.Infof("%d errors returned: %s", len(errors), strings.Join(errors, ", "))
	logrus.Infof("Allowed: %t", allowed)
	return response
}

type MetadataReport struct {
	admissionv1.AdmissionRequest
	NamespaceMetadata map[string]any `json:"namespaceMetadata,omitempty"`
}

func getRequestReport(req admission.Request, namespaceMetadata map[string]any) (models.ReportInfo, error) {
	if req.Operation == "DELETE" {
		req.Object = req.OldObject // DELETE requests don't have an object
	}
	metadataReport := MetadataReport{AdmissionRequest: req.AdmissionRequest, NamespaceMetadata: namespaceMetadata}
	contents, err := json.Marshal(&metadataReport)
	return models.ReportInfo{
		Report:   "metadata",
		Version:  "0.2.0",
		Contents: contents,
	}, err
}

func processInputYAML(ctx context.Context, iConfig models.InsightsConfig, config models.Configuration, decoded map[string]any, req admission.Request, namespaceMetadata map[string]any) (bool, []string, []string, error) {
	logrus.Debugf("Processing with config %+v", config)
	metadataReport, err := getRequestReport(req, namespaceMetadata)
	if err != nil {
		logrus.Errorf("Error marshaling admission request")
		return false, nil, nil, err
	}
	reports := []models.ReportInfo{metadataReport}
	if config.Reports.Polaris && len(req.Object.Raw) > 0 {
		logrus.Info("Running Polaris")
		// Scan manifests with Polaris
		polarisConfig := *config.Polaris
		polarisReport, err := polaris.GetPolarisReport(ctx, polarisConfig, req.Object.Raw)
		if err != nil {
			logrus.Errorf("Error while running Polaris: %v", err)
			return false, nil, nil, err
		}
		reports = append(reports, polarisReport)
	}

	if config.Reports.OPA {
		logrus.Info("Running OPA")
		opaReport, err := opa.ProcessOPA(ctx, decoded, req, config, iConfig)
		if err != nil {
			logrus.Errorf("Error while running OPA: %v", err)
			return false, nil, nil, err
		}
		reports = append(reports, opaReport)
	}

	if config.Reports.Pluto && len(req.Object.Raw) > 0 {
		logrus.Info("Running Pluto")
		userTargetVersionsStr := os.Getenv("PLUTO_TARGET_VERSIONS")
		userTargetVersions, err := pluto.ParsePlutoTargetVersions(userTargetVersionsStr)
		if err != nil {
			logrus.Errorf("unable to parse pluto target versions %q: %v", userTargetVersionsStr, err)
			return false, nil, nil, err
		}
		plutoReport, err := pluto.ProcessPluto(req.Object.Raw, userTargetVersions)
		if err != nil {
			logrus.Errorf("Error while running Pluto: %v", err)
			return false, nil, nil, err
		}
		reports = append(reports, plutoReport)
	}

	results, warnings, errors, err := sendResults(iConfig, reports)
	if err != nil {
		return false, nil, nil, err
	}
	return results, warnings, errors, nil
}

// ValidateIfControllerMatches checks if a child object is controlled by a parent object
func ValidateIfControllerMatches(child map[string]any, controller map[string]any) error {
	if child["metadata"].(map[string]any)["ownerReferences"].([]any)[0].(map[string]any)["uid"] != controller["metadata"].(map[string]any)["uid"] {
		return fmt.Errorf("controller does not match ownerReference uid")
	}
	if child["metadata"].(map[string]any)["namespace"].(string) != controller["metadata"].(map[string]any)["namespace"].(string) {
		return fmt.Errorf("controller namespace %s does not match ownerReference namespace %s", controller["metadata"].(map[string]any)["namespace"], child["metadata"].(map[string]any)["ownerReferences"].([]any)[0].(map[string]any)["namespace"])
	}
	if child["metadata"].(map[string]any)["ownerReferences"].([]any)[0].(map[string]any)["name"].(string) != controller["metadata"].(map[string]any)["name"].(string) {
		return fmt.Errorf("controller name %s does not match ownerReference name %s", controller["metadata"].(map[string]any)["name"], child["metadata"].(map[string]any)["ownerReferences"].([]any)[0].(map[string]any)["name"])
	}
	if !lo.Contains(controllerValidKinds, controller["kind"].(string)) {
		return fmt.Errorf("controller kind %s is not a valid controller kind", controller["kind"].(string))
	}
	childContainers := getChildContainers(child)
	controllerContainers := getControllerContainers(controller)
	/*
		if len(childContainers) != len(controllerContainers) {
			return fmt.Errorf("number of controller container does not match child containers")
		}
		childContainerNames := lo.Map(childContainers, func(container any, _ int) string {
			return getContainerKey(container.(map[string]any))
		})
		controllerContainerNames := lo.Map(controllerContainers, func(container any, _ int) string {
			return getContainerKey(container.(map[string]any))
		})
		for _, childContainerName := range childContainerNames {
			if !lo.Contains(controllerContainerNames, childContainerName) {
				return fmt.Errorf("controller does not match child containers names")
			}
		}
		err := validateSecurityContext(childContainers, controllerContainers)
		if err != nil {
			return err
		}
	*/
	err := validateContainersSpec(childContainers, controllerContainers)
	if err != nil {
		return err
	}

	return nil
}

func getContainerKey(container map[string]any) string {
	return fmt.Sprintf("%s/%s/%s", container["name"], container["image"], container["tag"])
}

func getChildContainers(child map[string]any) []any {
	if _, ok := child["spec"].(map[string]any)["containers"]; ok {
		return child["spec"].(map[string]any)["containers"].([]any)
	} else if _, ok := child["spec"].(map[string]any)["jobTemplate"]; ok {
		return child["spec"].(map[string]any)["jobTemplate"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)
	}
	return child["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)
}

func getControllerContainers(controller map[string]any) []any {
	if _, ok := controller["spec"].(map[string]any)["jobTemplate"]; ok {
		return controller["spec"].(map[string]any)["jobTemplate"].(map[string]any)["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)
	}
	return controller["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)
}

func validateSecurityContext(childContainers, controllerContainers []any) error {
	childContainerSecurityContext := map[string]string{}
	for _, container := range childContainers {
		jsonSecurityContext, err := json.Marshal(container.(map[string]any)["securityContext"])
		if err != nil {
			logrus.Error(err, "Error marshaling child securityContext")
			return err
		}
		childContainerSecurityContext[getContainerKey(container.(map[string]any))] = string(jsonSecurityContext)
	}
	controllerContainersSecurityContext := map[string]string{}
	for _, container := range controllerContainers {
		jsonSecurityContext, err := json.Marshal(container.(map[string]any)["securityContext"])
		if err != nil {
			logrus.Error(err, "Error marshaling controller securityContext")
			return err
		}
		controllerContainersSecurityContext[getContainerKey(container.(map[string]any))] = string(jsonSecurityContext)
	}
	for key, childContainerSecurityContext := range childContainerSecurityContext {
		controllerSecurityContext := controllerContainersSecurityContext[key]
		if childContainerSecurityContext != controllerSecurityContext {
			logrus.Infof("child container key: %s", key)
			logrus.Infof("child container securityContext: %s", childContainerSecurityContext)
			logrus.Infof("controller container securityContext: %s", controllerSecurityContext)
			return fmt.Errorf("controller does not match child containers securityContext")
		}
	}
	return nil
}

func validateContainersSpec(childContainers, controllerContainers []any) error {
	childContainerJSON := map[string]string{}
	for _, container := range childContainers {
		jsonData, err := json.Marshal(container.(map[string]any))
		if err != nil {
			logrus.Error(err, "Error marshaling child container")
			return err
		}
		childContainerJSON[getContainerKey(container.(map[string]any))] = string(jsonData)
	}
	controllerContainersJSON := map[string]string{}
	for _, container := range controllerContainers {
		jsonData, err := json.Marshal(container.(map[string]any))
		if err != nil {
			logrus.Error(err, "Error marshaling controller container")
			return err
		}
		controllerContainersJSON[getContainerKey(container.(map[string]any))] = string(jsonData)
	}
	for key, childContainerJSON := range childContainerJSON {
		controllerJSON := controllerContainersJSON[key]
		if childContainerJSON != childContainerJSON {
			logrus.Infof("child container key: %s", key)
			logrus.Infof("child container securityContext: %s", childContainerJSON)
			logrus.Infof("controller container securityContext: %s", controllerJSON)
			return fmt.Errorf("controller does not match child containers securityContext")
		}
	}
	return nil
}

var controllerValidKinds = []string{"Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "CronJob", "Job"}

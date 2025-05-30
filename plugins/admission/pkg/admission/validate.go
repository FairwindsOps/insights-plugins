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

	"github.com/fairwindsops/controller-utils/pkg/controller"
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
	if v == nil {
		return true
	}
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
	if v == nil {
		return nil
	}
	v.decoder = &d
	return nil
}

// InjectConfig injects the config.
func (v *Validator) InjectConfig(c models.Configuration) error {
	if v == nil {
		return nil
	}
	v.config = &c
	return nil
}

func (v *Validator) handleInternal(ctx context.Context, req admission.Request) (bool, []string, []string, error) {
	username := req.UserInfo.Username
	if lo.Contains(v.iConfig.IgnoreUsernames, username) {
		msg := fmt.Sprintf("Insights admission controller is ignoring service account %s.", username)
		return true, []string{msg}, nil, nil
	}
	if v.config == nil {
		return true, nil, nil, nil
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
		allOwnersValid := true
		for _, ownerReference := range ownerReferences {
			ownerReference := ownerReference.(map[string]any)
			client := kube.GetKubeClient()
			ctrl, err := client.GetObject(ctx, req.Namespace, ownerReference["kind"].(string), ownerReference["apiVersion"].(string), ownerReference["name"].(string), client.DynamicInterface, client.RestMapper)
			if err != nil {
				allOwnersValid = false
				logrus.Infof("error retrieving owner for object %s - running checks: %v", req.Name, err)
				break
			} else {
				err = controller.ValidateIfControllerMatches(decoded, ctrl.Object)
				if err != nil {
					allOwnersValid = false
					logrus.Infof("object %s has an owner but the owner is invalid - running checks: %v", req.Name, err)
					break
				}
			}
		}
		if allOwnersValid {
			logrus.Infof("object %s has owner(s) and the owner(s) are valid - skipping", req.Name)
			return true, nil, nil, nil
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
	metadataReport, err := getRequestReport(req, namespaceMetadata)
	if err != nil {
		logrus.Errorf("Error marshaling admission request")
		return false, nil, nil, err
	}
	reports := []models.ReportInfo{metadataReport}
	if config.Reports.Polaris && len(req.Object.Raw) > 0 && config.Polaris != nil {
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

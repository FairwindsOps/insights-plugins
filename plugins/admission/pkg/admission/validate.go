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
	logrus.Info("Injecting config--------------------------------------")
	v.decoder = &d
	return nil
}

// InjectConfig injects the config.
func (v *Validator) InjectConfig(c models.Configuration) error {
	logrus.Info("Injecting config")
	logrus.Info("Injecting config--------------------------------------")
	v.config = &c
	return nil
}

func (v *Validator) handleInternal(ctx context.Context, req admission.Request) (bool, []string, []string, error) {
	var decoded map[string]interface{}
	username := req.UserInfo.Username
	logrus.Infof("Using service account %s is being ignored by configuration", username)
	fmt.Println(username)
	logrus.Infof("Ignoring usernames=%s", v.iConfig.IgnoreUsernames)
	fmt.Println(v.iConfig.IgnoreUsernames)
	if lo.Contains(v.iConfig.IgnoreUsernames, username) {
		msg := fmt.Sprintf("Service account %s is being ignored by configuration", username)
		return true, []string{msg}, nil, nil
	}
	err := json.Unmarshal(req.Object.Raw, &decoded)
	if err != nil {
		logrus.Errorf("Error unmarshaling JSON")
		return false, nil, nil, err
	}
	ownerReferences, ok := decoded["metadata"].(map[string]interface{})["ownerReferences"].([]interface{})
	if ok && len(ownerReferences) > 0 {
		logrus.Infof("Object has an owner - skipping")
		return true, nil, nil, nil
	}

	var namespaceMetadata map[string]any
	if namespace, ok := decoded["metadata"].(map[string]interface{})["namespace"].(string); ok && namespace != "" {
		namespaceMetadata, err = getNamespaceMetadata(v.clientset, namespace)
		if err != nil {
			return false, nil, nil, err
		}
	}

	logrus.Debugf("Processing with config %+v", v.config)
	metadataReport, err := getRequestReport(req, namespaceMetadata)
	if err != nil {
		logrus.Errorf("Error marshaling admission request")
		return false, nil, nil, err
	}
	return processInputYAML(ctx, v.iConfig, *v.config, req.Object.Raw, decoded, req.AdmissionRequest.Name, req.AdmissionRequest.Namespace, req.AdmissionRequest.RequestKind.Kind, req.AdmissionRequest.RequestKind.Group, metadataReport)
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
	logrus.Infof("Starting %s request for %s%s/%s %s in namespace %s", req.Operation, req.RequestKind.Group, req.RequestKind.Version, req.RequestKind.Kind, req.Name, req.Namespace)
	allowed, warnings, errors, err := v.handleInternal(ctx, req)
	if err != nil {
		logrus.Errorf("-----------------Error validating request: %v", err)
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
		response.Warnings = warnings
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
	metadataReport := MetadataReport{AdmissionRequest: req.AdmissionRequest, NamespaceMetadata: namespaceMetadata}
	contents, err := json.Marshal(&metadataReport)
	return models.ReportInfo{
		Report:   "metadata",
		Version:  "0.2.0",
		Contents: contents,
	}, err
}

func processInputYAML(ctx context.Context, iConfig models.InsightsConfig, configurationObject models.Configuration, input []byte, decodedObject map[string]interface{}, name, namespace, kind, apiGroup string, metaReport models.ReportInfo) (bool, []string, []string, error) {
	reports := []models.ReportInfo{metaReport}
	if configurationObject.Reports.Polaris {
		logrus.Info("Running Polaris")
		// Scan manifests with Polaris
		polarisConfig := *configurationObject.Polaris
		polarisReport, err := polaris.GetPolarisReport(ctx, polarisConfig, input)
		if err != nil {
			return false, nil, nil, err
		}
		reports = append(reports, polarisReport)
	}

	if configurationObject.Reports.OPA {
		logrus.Info("Running OPA")
		opaReport, err := opa.ProcessOPA(ctx, decodedObject, name, apiGroup, kind, namespace, configurationObject, iConfig)
		if err != nil {
			return false, nil, nil, err
		}
		reports = append(reports, opaReport)
	}

	if configurationObject.Reports.Pluto {
		logrus.Info("Running Pluto")
		userTargetVersionsStr := os.Getenv("PLUTO_TARGET_VERSIONS")
		userTargetVersions, err := pluto.ParsePlutoTargetVersions(userTargetVersionsStr)
		if err != nil {
			logrus.Errorf("unable to parse pluto target versions %q: %v", userTargetVersionsStr, err)
			return false, nil, nil, err
		}
		plutoReport, err := pluto.ProcessPluto(input, userTargetVersions)
		if err != nil {
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

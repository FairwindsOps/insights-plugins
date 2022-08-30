package admission

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
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
	iConfig              models.InsightsConfig
	decoder              *admission.Decoder
	config               *models.Configuration
	webhookFailurePolicy webhookFailurePolicy
}

func NewValidator(iConfig models.InsightsConfig) *Validator {
	return &Validator{
		iConfig: iConfig,
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
	var err error
	var decoded map[string]interface{}

	err = json.Unmarshal(req.Object.Raw, &decoded)
	if err != nil {
		logrus.Errorf("Error unmarshaling JSON")
		return false, nil, nil, err
	}

	ownerReferences, ok := decoded["metadata"].(map[string]interface{})["ownerReferences"].([]interface{})

	if ok && len(ownerReferences) > 0 {
		logrus.Infof("Object has an owner - skipping")
		return true, nil, nil, nil
	}
	token := strings.TrimSpace(os.Getenv("FAIRWINDS_TOKEN"))

	logrus.Debugf("Processing with config %+v", v.config)
	metadata, err := getRequestReport(req)
	if err != nil {
		logrus.Errorf("Error marshaling admission request")
		return false, nil, nil, err
	}
	return processInputYAML(ctx, v.iConfig, *v.config, req.Object.Raw, decoded, token, req.AdmissionRequest.Name, req.AdmissionRequest.Namespace, req.AdmissionRequest.RequestKind.Kind, req.AdmissionRequest.RequestKind.Group, metadata)
}

// Handle for Validator to run validation checks.
func (v *Validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logrus.Infof("Starting %s request for %s%s/%s %s in namespace %s",
		req.Operation,
		req.RequestKind.Group,
		req.RequestKind.Version,
		req.RequestKind.Kind,
		req.Name,
		req.Namespace)
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
	logrus.Infof("%d warnings returned: %s", len(warnings), strings.Join(warnings, ", "))
	logrus.Infof("%d errors returned: %s", len(errors), strings.Join(errors, ", "))
	logrus.Infof("Allowed: %t", allowed)
	return response
}

func getRequestReport(req admission.Request) (models.ReportInfo, error) {
	report := models.ReportInfo{
		Report:  "metadata",
		Version: "0.1.0",
	}
	var err error
	report.Contents, err = json.Marshal(&req.AdmissionRequest)
	return report, err
}

func processInputYAML(ctx context.Context, iConfig models.InsightsConfig, configurationObject models.Configuration, input []byte, decodedObject map[string]interface{}, token, name, namespace, kind, apiGroup string, metaReport models.ReportInfo) (bool, []string, []string, error) {
	reports := []models.ReportInfo{
		metaReport,
	}
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
		opaReport, err := opa.ProcessOPA(ctx, decodedObject, name, apiGroup, kind, namespace, configurationObject)
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

	results, warnings, errors, err := sendResults(iConfig, reports, token)
	if err != nil {
		return false, nil, nil, err
	}
	return results, warnings, errors, nil
}

package admission

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/fairwindsops/insights-plugins/admission/pkg/models"
	"github.com/fairwindsops/insights-plugins/admission/pkg/opa"
	"github.com/fairwindsops/insights-plugins/admission/pkg/polaris"
)

type Validator struct {
	decoder *admission.Decoder
	Config  models.Configuration
}

// InjectDecoder injects the decoder.
func (v *Validator) InjectDecoder(d *admission.Decoder) error {
	logrus.Info("Injecting decoder")
	v.decoder = d
	return nil
}

func (v *Validator) handleInternal(ctx context.Context, req admission.Request) (bool, []string, []string, error) {
	var err error
	var decoded map[string]interface{}
	err = json.Unmarshal(req.Object.Raw, &decoded)
	if err != nil {
		return false, nil, nil, err
	}

	ownerReferences, ok := decoded["metadata"].(map[string]interface{})["ownerReferences"].([]interface{})

	if ok && len(ownerReferences) > 0 {
		return true, nil, nil, nil
	}
	token := strings.TrimSpace(os.Getenv("FAIRWINDS_TOKEN"))

	logrus.Infof("Processing with config %+v", v.Config)
	return processInputYAML(ctx, v.Config, req.Object.Raw, decoded, token, req.AdmissionRequest.Name, req.AdmissionRequest.Namespace, req.AdmissionRequest.RequestKind.Kind, req.AdmissionRequest.RequestKind.Group)
}

// Handle for Validator to run validation checks.
func (v *Validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logrus.Info("Starting request")
	allowed, warnings, errors, err := v.handleInternal(ctx, req)
	if err != nil {
		logrus.Errorf("Error validating request: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}
	response := admission.ValidationResponse(allowed, strings.Join(errors, "\n"))
	logrus.Infof("Warnings returned: %+v", warnings)
	return response
}

func processInputYAML(ctx context.Context, configurationObject models.Configuration, input []byte, decodedObject map[string]interface{}, token, name, namespace, kind, apiGroup string) (bool, []string, []string, error) {
	reports := make([]models.ReportInfo, 0)
	if configurationObject.Reports.Polaris {
		logrus.Info("Running Polaris")
		// Scan manifests with Polaris
		polarisConfig := *configurationObject.Polaris
		polarisReport, err := polaris.GetPolarisReport(ctx, polarisConfig, kind, input)
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

	// TODO add Pluto report
	results, warnings, errors, err := SendResults(reports, token)
	if err != nil {
		return false, nil, nil, err
	}
	return results, warnings, errors, nil
}

package polaris

import (
	"encoding/json"
	"os"

	polarisconfiguration "github.com/fairwindsops/polaris/pkg/config"
	"github.com/fairwindsops/polaris/pkg/kube"
	"github.com/fairwindsops/polaris/pkg/validator"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/models"
	corev1 "k8s.io/api/core/v1"
)

var config polarisconfiguration.Configuration

func init() {
	configPath := "examples/polaris.yaml"
	var err error

	config, err = polarisconfiguration.ParseFile(configPath)
	if err != nil {
		logrus.Errorf("Error parsing config at %s: %v", configPath, err)
		os.Exit(1)
	}
}

// GetPolarisReport returns the polaris report for the provided manifest.
func GetPolarisReport(manifest []byte) (models.ReportInfo, error) {
	report := models.ReportInfo{
		Report: "polaris",
	}
	// Scan with Polaris
	controller, err := kube.NewGenericResourceFromBytes(manifest)
	if err != nil {
		return report, err
	}
	controllerResult, err := validator.ApplyAllSchemaChecks(&config, nil, controller)
	if err != nil {
		return report, err
	}

	report.Version = validator.PolarisOutputVersion
	auditData := validator.AuditData{
		PolarisOutputVersion: validator.PolarisOutputVersion,
		Results:              []validator.Result{controllerResult},
	}
	bytes, err := json.Marshal(auditData)
	if err != nil {
		return report, err
	}
	report.Contents = bytes
	return report, nil
}

func GetPolarisValidateResults(kind string, decoder *admission.Decoder, req admission.Request, config polarisconfiguration.Configuration) (*validator.Result, error) {
	var controller kube.GenericResource
	var err error
	if kind == "Pod" {
		pod := corev1.Pod{}
		err := decoder.Decode(req, &pod)
		if err != nil {
			return nil, err
		}
		if len(pod.ObjectMeta.OwnerReferences) > 0 {
			logrus.Infof("Allowing owned pod %s/%s to pass through webhook", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
			return nil, nil
		}
		controller, err = kube.NewGenericResourceFromPod(pod, pod)
	} else {
		controller, err = kube.NewGenericResourceFromBytes(req.Object.Raw)
	}
	if err != nil {
		return nil, err
	}
	// TODO: consider enabling multi-resource checks
	controllerResult, err := validator.ApplyAllSchemaChecks(&config, nil, controller)
	if err != nil {
		return nil, err
	}
	return &controllerResult, nil
}

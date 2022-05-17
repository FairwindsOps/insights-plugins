package admission

import (
	"context"

	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/polaris"
	"github.com/fairwindsops/polaris/pkg/mutation"
	"github.com/sirupsen/logrus"
	"gomodules.xyz/jsonpatch/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Mutator is the entry point for the admission webhook.
type Mutator struct {
	decoder *admission.Decoder
	config  *models.Configuration
}

// InjectConfig injects the config.
func (m *Mutator) InjectConfig(c models.Configuration) error {
	logrus.Info("Injecting config")
	m.config = &c
	return nil
}

func (m *Mutator) mutate(req admission.Request) ([]jsonpatch.Operation, error) {
	results, err := polaris.GetPolarisValidateResults(req.AdmissionRequest.Kind.Kind, m.decoder, req, *m.config.Polaris)
	if err != nil {
		return nil, err
	}
	patches, _ := mutation.GetMutationsAndCommentsFromResult(results)
	return patches, nil
}

// Handle for Validator to run validation checks.
func (m *Mutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logrus.Infof("Starting %s request for %s%s/%s %s in namespace %s",
		req.Operation,
		req.RequestKind.Group,
		req.RequestKind.Version,
		req.RequestKind.Kind,
		req.Name,
		req.Namespace)
	patches, err := m.mutate(req)
	if err != nil {
		return admission.Errored(403, err)
	}
	if patches == nil {
		return admission.Allowed("Allowed")
	}
	return admission.Patched("", patches...)
}

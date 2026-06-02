package admission

import (
	"context"

	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/models"
	"github.com/fairwindsops/polaris/pkg/mutation"
	polariswebhook "github.com/fairwindsops/polaris/pkg/webhook"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"
)

// Mutator is the entry point for the admission webhook.
type Mutator struct {
	decoder *admission.Decoder
	config  *models.Configuration
}

// InjectConfig injects the config.
func (m *Mutator) InjectConfig(c models.Configuration) error {
	if m == nil {
		return nil
	}
	m.config = &c
	return nil
}

func (m *Mutator) mutate(ctx context.Context, req admission.Request) (original, mutated []byte, err error) {
	if m == nil {
		return nil, nil, nil
	}
	if m.config == nil || m.config.Polaris == nil {
		return nil, nil, nil
	}
	results, kubeResources, err := polariswebhook.GetValidatedResults(ctx, req.AdmissionRequest.Kind.Kind, m.decoder, req, *m.config.Polaris)
	if err != nil {
		return nil, nil, err
	}
	if results == nil || len(results.Results) == 0 {
		return nil, nil, nil
	}
	patches := mutation.GetMutationsFromResult(results)
	if len(patches) == 0 {
		return nil, nil, nil
	}
	originalYaml, err := yaml.JSONToYAML(kubeResources.OriginalObjectJSON)
	if err != nil {
		return nil, nil, err
	}
	mutatedYamlStr, err := mutation.ApplyAllMutations(string(originalYaml), patches)
	if err != nil {
		return nil, nil, err
	}
	mutatedJSON, err := yaml.YAMLToJSON([]byte(mutatedYamlStr))
	if err != nil {
		return nil, nil, err
	}
	return kubeResources.OriginalObjectJSON, mutatedJSON, nil
}

// Handle for Validator to run validation checks.
func (m *Mutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if m == nil {
		return admission.Allowed("Allowed")
	}
	if req.RequestKind == nil {
		return admission.Allowed("Allowed")
	}
	logrus.Infof("Starting %s request for %s%s/%s %s in namespace %s",
		req.Operation,
		req.RequestKind.Group,
		req.RequestKind.Version,
		req.RequestKind.Kind,
		req.Name,
		req.Namespace)
	original, mutated, err := m.mutate(ctx, req)
	if err != nil {
		logrus.Errorf("got an error getting patches: %v", err)
		return admission.Errored(403, err)
	}
	if original == nil {
		return admission.Allowed("Allowed")
	}
	return admission.PatchResponseFromRaw(original, mutated)
}

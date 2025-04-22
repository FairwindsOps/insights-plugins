package admission

import (
	"context"

	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/models"
	"github.com/fairwindsops/polaris/pkg/mutation"
	polariswebhook "github.com/fairwindsops/polaris/pkg/webhook"
	"github.com/sirupsen/logrus"
	"gomodules.xyz/jsonpatch/v2"
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

func (m *Mutator) mutate(req admission.Request) ([]jsonpatch.Operation, error) {
	if m == nil {
		return []jsonpatch.Operation{}, nil
	}
	if m.config == nil || m.config.Polaris == nil {
		return []jsonpatch.Operation{}, nil
	}
	logrus.Infof("-----Mutator got a config for %s", req.Name)
	results, kubeResources, err := polariswebhook.GetValidatedResults(req.AdmissionRequest.Kind.Kind, m.decoder, req, *m.config.Polaris)
	if err != nil {
		logrus.Errorf("xxxxxxxxx got an error getting results: %v", err)
		return nil, err
	}
	if results == nil || len(results.Results) == 0 {
		return []jsonpatch.Operation{}, nil
	}
	logrus.Info("======Mutator got results for ", req.Name, results.Results)
	patches := mutation.GetMutationsFromResult(results)
	if len(patches) == 0 {
		return []jsonpatch.Operation{}, nil
	}
	originalYaml, err := yaml.JSONToYAML(kubeResources.OriginalObjectJSON)
	if err != nil {
		logrus.Errorf("xxxxxxxxx got an error converting json to yaml: %v", err)
		return nil, err
	}
	logrus.Info("originalYaml====", string(originalYaml))
	logrus.Info("patches====", patches)
	mutatedYamlStr, err := mutation.ApplyAllMutations(string(originalYaml), patches)
	if err != nil {
		return nil, err
	}
	logrus.Info("mutatedYamlStr====", mutatedYamlStr)
	mutatedJson, err := yaml.YAMLToJSON([]byte(mutatedYamlStr))
	if err != nil {
		return nil, err
	}
	returnPatch, err := jsonpatch.CreatePatch(kubeResources.OriginalObjectJSON, mutatedJson)
	if err != nil {
		return nil, err
	}
	return returnPatch, err
}

// Handle for Validator to run validation checks.
func (m *Mutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if m == nil {
		logrus.Infof("Mutator got an empty Mutaror for %s", req.Name)
		return admission.Allowed("Allowed")
	}
	if req.RequestKind == nil {
		logrus.Infof("Mutator got an empty request kind for %s", req.Name)
		return admission.Allowed("Allowed")
	}
	logrus.Infof("Starting %s request for %s%s/%s %s in namespace %s",
		req.Operation,
		req.RequestKind.Group,
		req.RequestKind.Version,
		req.RequestKind.Kind,
		req.Name,
		req.Namespace)
	patches, err := m.mutate(req)
	if err != nil {
		logrus.Errorf("got an error getting patches: %v", err)
		return admission.Errored(403, err)
	}
	if len(patches) == 0 {
		logrus.Infof("no patches to apply for %s/%s: ALLOWED", req.RequestKind.Kind, req.Name)
		return admission.Allowed("Allowed")
	}
	return admission.Patched("", patches...)

}

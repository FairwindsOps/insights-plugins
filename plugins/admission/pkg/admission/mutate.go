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
	m.config = &c
	return nil
}

func (m *Mutator) mutate(req admission.Request) ([]jsonpatch.Operation, error) {
	logrus.Infof("mutating %s/%s", req.RequestKind.Kind, req.Name)
	results, kubeResources, err := polariswebhook.GetValidatedResults(req.AdmissionRequest.Kind.Kind, m.decoder, req, *m.config.Polaris)
	logrus.Infof("got %d results from polaris", len(results.Results))
	if err != nil {
		logrus.Errorf("got an error getting validated results: %v", err)
		return nil, err
	}
	logrus.Infof("polaris returned %d results during mutation of %s/%s: %v", len(results.Results), req.RequestKind.Kind, req.Name, *results)
	if len(results.Results) == 0 {
		logrus.Infof("no results to mutate for %s/%s", req.RequestKind.Kind, req.Name)
		return []jsonpatch.Operation{}, nil
	}
	patches := mutation.GetMutationsFromResult(results)
	if len(patches) == 0 {
		logrus.Infof("no patches to apply for %s/%s", req.RequestKind.Kind, req.Name)
		return []jsonpatch.Operation{}, nil
	}
	originalYaml, err := yaml.JSONToYAML(kubeResources.OriginalObjectJSON)
	if err != nil {
		logrus.Errorf("got an error converting original object to yaml: %v", err)
		return nil, err
	}
	mutatedYamlStr, err := mutation.ApplyAllMutations(string(originalYaml), patches)
	logrus.Info("string(originalYaml)))=====", string(originalYaml))
	logrus.Info("mutatedYamlStr=====", mutatedYamlStr)
	if err != nil {
		logrus.Errorf("got an error applying mutations: %v", err)
		return nil, err
	}
	mutatedJson, err := yaml.YAMLToJSON([]byte(mutatedYamlStr))
	if err != nil {
		logrus.Errorf("got an error converting mutated object to json: %v", err)
		return nil, err
	}
	returnPatch, err := jsonpatch.CreatePatch(kubeResources.OriginalObjectJSON, mutatedJson)
	if err != nil {
		logrus.Errorf("got an error creating json patch: %v", err)
		return nil, err
	}
	logrus.Debugf("the patch to mutate %s/%s is: %v", req.RequestKind.Kind, req.Name, returnPatch)
	return returnPatch, err
}

// Handle for Validator to run validation checks.
func (m *Mutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logrus.Infof("Mutaror starting %s request for %s%s/%s %s in namespace %s",
		req.Operation,
		req.RequestKind.Group,
		req.RequestKind.Version,
		req.RequestKind.Kind,
		req.Name,
		req.Namespace)
	logrus.Infof("OPERATION============== got %s request for %s/%s/%s", req.Operation, req.RequestKind.Kind, req.Name, req.Operation)
	patches, err := m.mutate(req)
	logrus.Infof("Mutator got %d patches for %s/%s", len(patches), req.RequestKind.Kind, req.Name)
	if err != nil {
		logrus.Errorf("got an error getting patches: %v", err)
		return admission.Errored(403, err)
	}
	if len(patches) == 0 {
		logrus.Infof("no patches to apply for %s/%s: ALLOWED", req.RequestKind.Kind, req.Name)
		return admission.Allowed("Allowed")
	}
	logrus.Infof("Mutator got %d patches for %s/%s", len(patches), req.RequestKind.Kind, req.Name)
	return admission.Patched("", patches...)
}

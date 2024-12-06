package opa

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/fairwindsops/insights-plugins/plugins/opa/pkg/kube"
	"github.com/fairwindsops/insights-plugins/plugins/opa/pkg/rego"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var fakeObj = unstructured.Unstructured{
	Object: map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]string{
				"foo": "bar",
			},
		},
	},
}

const basicRego = `
package fairwinds
labelblock[description] {
  provided := {label | input.metadata.labels[label]}
  required := {label | label := input.parameters.labels[_]}
  missing := required - provided
  found := required - missing
  count(found) > 0
  description := sprintf("label %v is present", [found])
}
`

const returnFull = `
package fairwinds
full[actionItem] {
  actionItem := {
    "description": "desc",
	"title": "title",
	"remediation": "remediation",
	"severity": 0.5,
	"category": "Security",
  }
}
`

const returnEmpty = `
package fairwinds
full[actionItem] {
  actionItem := {}
}
`

const brokenRego = `
package fairwinds
labelblock[description] {
  provided := foobar
  description := "shouldn't get here"
}
`

const regoWithK8s = `
package fairwinds
k8s[actionItem] {
  deps := kubernetes("apps", "Deployment")
  has_item := count(deps) > 0
  has_item == true
  first := deps[0]
  meta := first.metadata
  actionItem := concat(" ", ["Found a deployment in", meta.namespace])
}
`

const regoWithIncorrectInsightsInfo = `
package fairwinds
requestinvalidinsightsinfo[description] {
  description := insightsinfo("invalidInsightsInfoRequested")
}
`

const regoWithInsightsInfo = `
package fairwinds
requestinsightsinfo[description] {
  description := sprintf("the context is %v, the cluster is %v and admissionRequest is %v", [insightsinfo("context"), insightsinfo("cluster"), insightsinfo("admissionRequest")])
}
`

const regoWithInsightsInfoAdmissionOpField = `
package fairwinds
requestinsightsinfo[description] {
  description := sprintf("the context is %v, the cluster is %v and admissionRequest is %v", [insightsinfo("context"), insightsinfo("cluster"), insightsinfo("admissionRequest").operation])
}
`

const maxReplicas = `
package fairwinds

foo := {"s": "foo"}

envMaxReplicasDeployments {
    print(foo.s)
    input.kind == "Deployment"
    env_suffix := array.reverse(split(input.metadata.namespace, "-"))[0]
    replicas := input.spec.replicas

    actionItem := {
      "title": "Non-production environment replica count exceeds maximum",
      "description": sprintf("The Deployment %v in the %v environment replicas exceed the maximum replica count for this environment.", [input.metadata.name, env_suffix]),
      "severity": 0.5,
      "remediation": "Reduce the number of replicas",
      "category": "Reliability"
    }
}
`

func TestOPAParseFail(t *testing.T) {
	kube.SetFakeClient()
	ctx := context.TODO()

	params := map[string]interface{}{}
	_, err := runRegoForItem(ctx, brokenRego, params, fakeObj.Object, &rego.InsightsInfo{})
	assert.Error(t, err)
}

func TestOPAParseInsightsInfoFail(t *testing.T) {
	kube.SetFakeClient()
	ctx := context.TODO()

	params := map[string]interface{}{}
	_, err := runRegoForItem(ctx, regoWithIncorrectInsightsInfo, params, fakeObj.Object, &rego.InsightsInfo{})
	assert.Error(t, err)
}

func TestReturnDescription(t *testing.T) {
	kube.SetFakeClient()
	ctx := context.TODO()
	details := OutputFormat{}

	params := map[string]interface{}{}
	results, err := runRegoForItem(ctx, basicRego, params, fakeObj.Object, &rego.InsightsInfo{})
	assert.NoError(t, err)
	ais, err := processResults(fakeObj.GetName(), fakeObj.GetKind(), fakeObj.GetNamespace(), results, "my-test", details)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(ais))

	params["labels"] = []string{"foo"}
	results, err = runRegoForItem(ctx, basicRego, params, fakeObj.Object, &rego.InsightsInfo{})
	assert.NoError(t, err)
	ais, err = processResults(fakeObj.GetName(), fakeObj.GetKind(), fakeObj.GetNamespace(), results, "my-test", details)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ais))
	assert.Equal(t, defaultSeverity, ais[0].Severity)
	assert.Equal(t, defaultTitle, ais[0].Title)
	assert.Equal(t, defaultRemediation, ais[0].Remediation)
	assert.Equal(t, defaultCategory, ais[0].Category)
	assert.Equal(t, "label {\"foo\"} is present", ais[0].Description)

	params = map[string]interface{}{}
	results, err = runRegoForItem(ctx, regoWithInsightsInfo, params, fakeObj.Object, &rego.InsightsInfo{InsightsContext: "Agent", Cluster: "us-east-1"})
	assert.NoError(t, err)
	ais, err = processResults(fakeObj.GetName(), fakeObj.GetKind(), fakeObj.GetNamespace(), results, "my-test", details)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ais))
	assert.Equal(t, "the context is Agent, the cluster is us-east-1 and admissionRequest is null", ais[0].Description)

	params = map[string]interface{}{}
	req := &admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			Name:      "name",
			Namespace: "ns",
			RequestKind: &metav1.GroupVersionKind{
				Kind:  "Pod",
				Group: "Group",
			},
		},
	}
	results, err = runRegoForItem(ctx, regoWithInsightsInfoAdmissionOpField, params, fakeObj.Object, &rego.InsightsInfo{InsightsContext: "Agent", Cluster: "us-east-1", AdmissionRequest: req})
	assert.NoError(t, err)
	ais, err = processResults(fakeObj.GetName(), fakeObj.GetKind(), fakeObj.GetNamespace(), results, "my-test", details)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ais))
	assert.Equal(t, "the context is Agent, the cluster is us-east-1 and admissionRequest is CREATE", ais[0].Description)
}

func TestExampleFiles(t *testing.T) {
	kube.SetFakeClient()
	err := filepath.Walk("../../examples", func(path string, info os.FileInfo, err error) error {
		if info.Name() != "policy.rego" {
			return nil
		}
		t.Log("testing", path)
		file, err := os.Open(path)
		if err != nil {
			panic(err)
		}
		defer file.Close()
		bytes, err := ioutil.ReadAll(file)
		if err != nil {
			panic(err)
		}

		regoString := string(bytes)
		ctx := context.TODO()
		params := map[string]interface{}{}
		_, err = runRegoForItem(ctx, regoString, params, fakeObj.Object, &rego.InsightsInfo{})
		return err
	})
	assert.NoError(t, err)
}

func TestReturnFull(t *testing.T) {
	kube.SetFakeClient()
	ctx := context.TODO()
	details := OutputFormat{}

	params := map[string]interface{}{}
	results, err := runRegoForItem(ctx, returnFull, params, fakeObj.Object, &rego.InsightsInfo{})
	assert.NoError(t, err)
	ais, err := processResults(fakeObj.GetName(), fakeObj.GetKind(), fakeObj.GetNamespace(), results, "my-test", details)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ais))
	assert.Equal(t, "title", ais[0].Title)
	assert.Equal(t, "desc", ais[0].Description)
	assert.Equal(t, "remediation", ais[0].Remediation)
	assert.Equal(t, 0.5, ais[0].Severity)
	assert.Equal(t, "Security", ais[0].Category)

	defaultTitle := "default title"
	defaultSeverity := 1.0
	defaultRemediation := "default remediation"
	defaultCategory := "Reliability"
	defaultDescription := "default description"
	details = OutputFormat{
		Title:       &defaultTitle,
		Severity:    &defaultSeverity,
		Remediation: &defaultRemediation,
		Description: &defaultDescription,
		Category:    &defaultCategory,
	}

	results, err = runRegoForItem(ctx, returnFull, params, fakeObj.Object, &rego.InsightsInfo{})
	assert.NoError(t, err)
	ais, err = processResults(fakeObj.GetName(), fakeObj.GetKind(), fakeObj.GetNamespace(), results, "my-test", details)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ais))
	assert.Equal(t, "title", ais[0].Title)
	assert.Equal(t, "desc", ais[0].Description)
	assert.Equal(t, "remediation", ais[0].Remediation)
	assert.Equal(t, 0.5, ais[0].Severity)
	assert.Equal(t, "Security", ais[0].Category)

	results, err = runRegoForItem(ctx, returnEmpty, params, fakeObj.Object, &rego.InsightsInfo{})
	assert.NoError(t, err)
	ais, err = processResults(fakeObj.GetName(), fakeObj.GetKind(), fakeObj.GetNamespace(), results, "my-test", details)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ais))
	assert.Equal(t, defaultTitle, ais[0].Title)
	assert.Equal(t, defaultDescription, ais[0].Description)
	assert.Equal(t, defaultRemediation, ais[0].Remediation)
	assert.Equal(t, defaultSeverity, ais[0].Severity)
	assert.Equal(t, defaultCategory, ais[0].Category)
}

func TestK8sAPI(t *testing.T) {
	kube.SetFakeClient()
	ctx := context.TODO()
	details := OutputFormat{}

	params := map[string]interface{}{}
	results, err := runRegoForItem(ctx, regoWithK8s, params, fakeObj.Object, &rego.InsightsInfo{})
	assert.NoError(t, err)
	ais, err := processResults(fakeObj.GetName(), fakeObj.GetKind(), fakeObj.GetNamespace(), results, "my-test", details)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(ais))

	kube.AddFakeDeployment()
	results, err = runRegoForItem(ctx, regoWithK8s, params, fakeObj.Object, &rego.InsightsInfo{})
	assert.NoError(t, err)
	ais, err = processResults(fakeObj.GetName(), fakeObj.GetKind(), fakeObj.GetNamespace(), results, "my-test", details)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ais))
	assert.Equal(t, "Found a deployment in test", ais[0].Description)
}

func TestMaxReplicas(t *testing.T) {
	kube.SetFakeClient()
	ctx := context.TODO()
	params := map[string]interface{}{}
	_, err := runRegoForItem(ctx, maxReplicas, params, fakeObj.Object, &rego.InsightsInfo{})
	assert.NoError(t, err)
}

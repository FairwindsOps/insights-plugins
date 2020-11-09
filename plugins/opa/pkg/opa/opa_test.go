package opa

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/fairwindsops/insights-plugins/opa/pkg/kube"
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

func TestOPAParseFail(t *testing.T) {
	kube.SetFakeClient()
	ctx := context.TODO()

	params := map[string]interface{}{}
	_, err := runRegoForItem(ctx, brokenRego, params, fakeObj.Object)
	assert.Error(t, err)
}

func TestReturnDescription(t *testing.T) {
	kube.SetFakeClient()
	ctx := context.TODO()
	details := OutputFormat{}

	params := map[string]interface{}{}
	results, err := runRegoForItem(ctx, basicRego, params, fakeObj.Object)
	assert.NoError(t, err)
	ais, err := processResults(fakeObj.GetName(), fakeObj.GetKind(), fakeObj.GetNamespace(), results, "my-test", details)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(ais))

	params["labels"] = []string{"foo"}
	results, err = runRegoForItem(ctx, basicRego, params, fakeObj.Object)
	assert.NoError(t, err)
	ais, err = processResults(fakeObj.GetName(), fakeObj.GetKind(), fakeObj.GetNamespace(), results, "my-test", details)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ais))
	assert.Equal(t, defaultSeverity, ais[0].Severity)
	assert.Equal(t, defaultTitle, ais[0].Title)
	assert.Equal(t, defaultRemediation, ais[0].Remediation)
	assert.Equal(t, defaultCategory, ais[0].Category)
	assert.Equal(t, "label {\"foo\"} is present", ais[0].Description)
}

func TestExampleFiles(t *testing.T) {
	kube.SetFakeClient()
	err := filepath.Walk("../../examples", func(path string, info os.FileInfo, err error) error {
		if info.Name() != "policy.yaml" {
			return nil
		}
		var object map[string]interface{}
		file, err := os.Open(path)
		if err != nil {
			return err
		}

		defer file.Close()
		bytes, _ := ioutil.ReadAll(file)
		err = yaml.Unmarshal(bytes, &object)
		if err != nil {
			return err
		}

		rego := object["rego"].(string)

		ctx := context.TODO()

		params := map[string]interface{}{}
		_, err = runRegoForItem(ctx, rego, params, fakeObj.Object)
		return err
	})
	assert.NoError(t, err)
}

func TestReturnFull(t *testing.T) {
	kube.SetFakeClient()
	ctx := context.TODO()
	details := OutputFormat{}

	params := map[string]interface{}{}
	results, err := runRegoForItem(ctx, returnFull, params, fakeObj.Object)
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

	results, err = runRegoForItem(ctx, returnFull, params, fakeObj.Object)
	assert.NoError(t, err)
	ais, err = processResults(fakeObj.GetName(), fakeObj.GetKind(), fakeObj.GetNamespace(), results, "my-test", details)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ais))
	assert.Equal(t, "title", ais[0].Title)
	assert.Equal(t, "desc", ais[0].Description)
	assert.Equal(t, "remediation", ais[0].Remediation)
	assert.Equal(t, 0.5, ais[0].Severity)
	assert.Equal(t, "Security", ais[0].Category)

	results, err = runRegoForItem(ctx, returnEmpty, params, fakeObj.Object)
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
	results, err := runRegoForItem(ctx, regoWithK8s, params, fakeObj.Object)
	assert.NoError(t, err)
	ais, err := processResults(fakeObj.GetName(), fakeObj.GetKind(), fakeObj.GetNamespace(), results, "my-test", details)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(ais))

	kube.AddFakeDeployment()
	results, err = runRegoForItem(ctx, regoWithK8s, params, fakeObj.Object)
	assert.NoError(t, err)
	ais, err = processResults(fakeObj.GetName(), fakeObj.GetKind(), fakeObj.GetNamespace(), results, "my-test", details)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ais))
	assert.Equal(t, "Found a deployment in test", ais[0].Description)
}

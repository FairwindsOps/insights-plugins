package opa

import (
	"strings"

	"github.com/thoas/go-funk"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// ActionItem represents an action item from a report
type ActionItem struct {
	ResourceNamespace string
	ResourceKind      string
	ResourceName      string
	Title             string
	Description       string
	Remediation       string
	EventType         string
	Severity          float64
	Category          string
}

type kubeClient struct {
	restMapper       meta.RESTMapper
	dynamicInterface dynamic.Interface
}

type customCheckInstance struct {
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              customCheckInstanceSpec
}

type customCheckInstanceSpec struct {
	Parameters      map[string]interface{}
	Targets         []kubeTarget
	Output          outputFormat
	CustomCheckName string
}

type kubeTarget struct {
	APIGroups []string `json:"apiGroups"`
	Kinds     []string
}

type outputFormat struct {
	Title       *string
	Severity    *float64
	Remediation *string
	Category    *string
}

type customCheck struct {
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              customCheckSpec
}

type customCheckSpec struct {
	AdditionalKubernetesData []kubeTarget
	Output                   outputFormat
	Rego                     string
}

type clusterCheckModel struct {
	Checks    []opaCustomCheck
	Instances []checkSetting
}

type opaCustomCheck struct {
	Name                     string
	Rego                     string
	Title                    *string
	Severity                 *float64
	Remediation              *string
	Category                 *string
	AdditionalKubernetesData []string
}

type checkSetting struct {
	CheckName      string
	Targets        []string
	AdditionalData struct {
		Name       string
		Output     outputFormat
		Parameters map[string]interface{}
	}
}

func (supposedInstance checkSetting) GetUnstructuredObject(namespace string) *unstructured.Unstructured {
	output := map[string]interface{}{}
	if supposedInstance.AdditionalData.Output.Remediation != nil {
		output["remedidation"] = supposedInstance.AdditionalData.Output.Remediation
	}
	if supposedInstance.AdditionalData.Output.Title != nil {
		output["title"] = supposedInstance.AdditionalData.Output.Title
	}
	if supposedInstance.AdditionalData.Output.Severity != nil {
		output["severity"] = supposedInstance.AdditionalData.Output.Severity
	}
	if supposedInstance.AdditionalData.Output.Category != nil {
		output["category"] = supposedInstance.AdditionalData.Output.Category
	}
	spec := map[string]interface{}{
		"customCheckName": supposedInstance.CheckName,
		"output":          output,
		"targets": funk.Map(supposedInstance.Targets, func(s string) map[string]interface{} {
			splitValues := strings.Split(s, "/")
			return map[string]interface{}{
				"apiGroups": []string{splitValues[0]},
				"kinds":     []string{splitValues[1]},
			}
		}).([]map[string]interface{}),
	}

	if supposedInstance.AdditionalData.Parameters != nil {
		spec["parameters"] = supposedInstance.AdditionalData.Parameters
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "CustomCheckInstance",
			"apiVersion": instanceGvr.Group + "/" + instanceGvr.Version,
			"metadata": map[string]interface{}{
				"name":      supposedInstance.AdditionalData.Name,
				"namespace": namespace,
			},
			"spec": spec,
		},
	}
}

func (supposedCheck opaCustomCheck) GetUnstructuredObject(namespace string) *unstructured.Unstructured {

	output := map[string]interface{}{}
	if supposedCheck.Remediation != nil {
		output["remedidation"] = supposedCheck.Remediation
	}
	if supposedCheck.Title != nil {
		output["title"] = supposedCheck.Title
	}
	if supposedCheck.Severity != nil {
		output["severity"] = supposedCheck.Severity
	}
	if supposedCheck.Category != nil {
		output["category"] = supposedCheck.Category
	}

	// TODO add owner ref
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "CustomCheck",
			"apiVersion": checkGvr.Group + "/" + checkGvr.Version,
			"metadata": map[string]interface{}{
				"name":      supposedCheck.Name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"rego":   supposedCheck.Rego,
				"output": output,
				"additionalKubernetesData": funk.Map(supposedCheck.AdditionalKubernetesData, func(s string) map[string]interface{} {
					splitValues := strings.Split(s, "/")
					return map[string]interface{}{
						"apiGroups": []string{splitValues[0]},
						"kinds":     []string{splitValues[1]},
					}
				}).([]map[string]interface{}),
			},
		},
	}
}

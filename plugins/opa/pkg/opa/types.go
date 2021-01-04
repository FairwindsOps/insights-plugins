package opa

import (
	"strings"

	"github.com/thoas/go-funk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

// CustomCheckInstance is an instance of a custom check
type CustomCheckInstance struct {
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              CustomCheckInstanceSpec
}

// CustomCheckInstanceSpec is the body of an instance of a custom check
type CustomCheckInstanceSpec struct {
	Parameters      map[string]interface{}
	Targets         []KubeTarget
	Output          OutputFormat
	CustomCheckName string
}

// KubeTarget is a mapping of kinds and API groups
type KubeTarget struct {
	APIGroups []string `json:"apiGroups"`
	Kinds     []string
}

type OutputFormat struct {
	Title       *string
	Severity    *float64
	Remediation *string
	Category    *string
	Description *string
}

func (o *OutputFormat) SetDefaults(others ...OutputFormat) {
	for _, other := range others {
		if o.Title == nil {
			o.Title = other.Title
		}
		if o.Severity == nil {
			o.Severity = other.Severity
		}
		if o.Remediation == nil {
			o.Remediation = other.Remediation
		}
		if o.Category == nil {
			o.Category = other.Category
		}
		if o.Description == nil {
			o.Description = other.Description
		}
	}
}

// CustomCheck is a custom OPA check.
type CustomCheck struct {
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              CustomCheckSpec
}

// CustomCheckSpec is the body of a Custom Check object
type CustomCheckSpec struct {
	AdditionalKubernetesData []KubeTarget
	Output                   OutputFormat
	Rego                     string
}

type clusterCheckModel struct {
	Checks    []OPACustomCheck
	Instances []CheckSetting
}

type OPACustomCheck struct {
	Name                     string
	Rego                     string
	Title                    *string
	Severity                 *float64
	Remediation              *string
	Category                 *string
	AdditionalKubernetesData []string
}

type CheckSetting struct {
	CheckName      string
	Targets        []string
	ClusterNames   []string
	AdditionalData struct {
		Name       string
		Output     OutputFormat
		Parameters map[string]interface{}
	}
}

func (instance CustomCheckInstance) MatchesTarget(apiGroup, kind string) bool {
	for _, target := range instance.Spec.Targets {
		for _, group := range target.APIGroups {
			for _, targetKind := range target.Kinds {
				if apiGroup == group && targetKind == kind {
					return true
				}
			}
		}
	}
	return false
}

func (supposedInstance CheckSetting) GetCustomCheckInstance() CustomCheckInstance {
	return CustomCheckInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: supposedInstance.AdditionalData.Name,
		},
		Spec: CustomCheckInstanceSpec{
			CustomCheckName: supposedInstance.CheckName,
			Output:          supposedInstance.AdditionalData.Output,
			Parameters:      supposedInstance.AdditionalData.Parameters,
			Targets: funk.Map(supposedInstance.Targets, func(s string) KubeTarget {
				splitValues := strings.Split(s, "/")
				return KubeTarget{
					APIGroups: []string{splitValues[0]},
					Kinds:     []string{splitValues[1]},
				}
			}).([]KubeTarget),
		},
	}
}

func (supposedInstance CheckSetting) GetUnstructuredObject(namespace string) *unstructured.Unstructured {
	output := map[string]interface{}{}
	if supposedInstance.AdditionalData.Output.Remediation != nil {
		output["remediation"] = supposedInstance.AdditionalData.Output.Remediation
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

func (supposedCheck OPACustomCheck) GetCustomCheck() CustomCheck {
	return CustomCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name: supposedCheck.Name,
		},
		Spec: CustomCheckSpec{
			Output: OutputFormat{
				Title:       supposedCheck.Title,
				Severity:    supposedCheck.Severity,
				Remediation: supposedCheck.Remediation,
				Category:    supposedCheck.Category,
			},
			Rego: supposedCheck.Rego,
		},
	}
}

func (supposedCheck OPACustomCheck) GetUnstructuredObject(namespace string) *unstructured.Unstructured {

	output := map[string]interface{}{}
	if supposedCheck.Remediation != nil {
		output["remediation"] = supposedCheck.Remediation
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
			},
		},
	}
}

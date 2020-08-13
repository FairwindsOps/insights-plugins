package main

import (
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

// Output is the format for the output file
type Output struct {
	ActionItems []ActionItem
}

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
		Properties map[string]interface{}
	}
}

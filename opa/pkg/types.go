package main

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ActionItem represents an action item from a report
type ActionItem struct {
	ResourceNamespace string
	ResourceKind      string
	ResourceName      string
	Title             string
	Description       string
	Remediation       string
	Severity          float64
	Category          string
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
	ApiGroups []string
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

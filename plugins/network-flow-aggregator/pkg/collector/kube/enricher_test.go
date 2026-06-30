package kube

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestWorkloadIdentityFromControllerDefaultsPodKind(t *testing.T) {
	top := unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name":      "kube-scheduler-ebpf-control-plane",
			"namespace": "kube-system",
		},
	}}

	id := workloadIdentityFromController(top, "kube-system", "kube-scheduler-ebpf-control-plane")
	if id.Kind != "Pod" {
		t.Fatalf("kind = %q, want Pod", id.Kind)
	}
	if id.Name != "kube-scheduler-ebpf-control-plane" {
		t.Fatalf("name = %q", id.Name)
	}
}

func TestWorkloadIdentityFromControllerKeepsDeployment(t *testing.T) {
	top := unstructured.Unstructured{Object: map[string]any{
		"kind": "Deployment",
		"metadata": map[string]any{
			"name":      "coredns",
			"namespace": "kube-system",
		},
	}}

	id := workloadIdentityFromController(top, "kube-system", "coredns-abc")
	if id.Kind != "Deployment" || id.Name != "coredns" {
		t.Fatalf("identity = %#v", id)
	}
}

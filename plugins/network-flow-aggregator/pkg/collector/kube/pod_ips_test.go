package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPodIPIndexSinglePod(t *testing.T) {
	idx := buildPodIPIndex([]*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "app", Name: "api-abc"},
			Status:     corev1.PodStatus{PodIP: "172.20.108.47", Phase: corev1.PodRunning},
		},
	})

	ref, ok := idx.lookup("172.20.108.47")
	if !ok {
		t.Fatal("expected match")
	}
	if ref.Namespace != "app" || ref.Name != "api-abc" {
		t.Fatalf("ref = %#v", ref)
	}
}

func TestPodIPIndexSkipsHostNetwork(t *testing.T) {
	idx := buildPodIPIndex([]*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "aws-node-xyz"},
			Spec:       corev1.PodSpec{HostNetwork: true},
			Status:     corev1.PodStatus{PodIP: "172.20.101.8", Phase: corev1.PodRunning},
		},
	})

	if _, ok := idx.lookup("172.20.101.8"); ok {
		t.Fatal("expected hostNetwork pod to be excluded")
	}
}

func TestPodIPIndexAmbiguousCollision(t *testing.T) {
	idx := buildPodIPIndex([]*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "aws-node-xyz"},
			Status:     corev1.PodStatus{PodIP: "172.20.101.8", Phase: corev1.PodRunning},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "kube-proxy-xyz"},
			Status:     corev1.PodStatus{PodIP: "172.20.101.8", Phase: corev1.PodRunning},
		},
	})

	if _, ok := idx.lookup("172.20.101.8"); ok {
		t.Fatal("expected ambiguous IP to be excluded")
	}
}

func TestPodIPIndexDualStack(t *testing.T) {
	idx := buildPodIPIndex([]*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "app", Name: "dual-abc"},
			Status: corev1.PodStatus{
				PodIP: "10.244.0.5",
				PodIPs: []corev1.PodIP{
					{IP: "10.244.0.5"},
					{IP: "fd00:10:244::5"},
				},
				Phase: corev1.PodRunning,
			},
		},
	})

	ref, ok := idx.lookup("10.244.0.5")
	if !ok || ref.Name != "dual-abc" {
		t.Fatalf("ipv4 lookup = %#v, ok=%v", ref, ok)
	}

	ref, ok = idx.lookup("fd00:10:244::5")
	if !ok || ref.Name != "dual-abc" {
		t.Fatalf("ipv6 lookup = %#v, ok=%v", ref, ok)
	}
}

func TestPodIPIndexSkipsTerminated(t *testing.T) {
	idx := buildPodIPIndex([]*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "app", Name: "done-job"},
			Status:     corev1.PodStatus{PodIP: "10.244.0.99", Phase: corev1.PodSucceeded},
		},
	})

	if _, ok := idx.lookup("10.244.0.99"); ok {
		t.Fatal("expected terminated pod to be excluded")
	}
}

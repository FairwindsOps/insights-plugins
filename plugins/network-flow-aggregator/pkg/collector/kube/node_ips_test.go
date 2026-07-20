package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNodeIPIndexLookup(t *testing.T) {
	idx := buildNodeIPIndex([]*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "node-a"},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: "10.0.1.5"},
					{Type: corev1.NodeExternalIP, Address: "203.0.113.10"},
					{Type: corev1.NodeHostName, Address: "node-a"},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "node-b"},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: "10.0.1.6"},
				},
			},
		},
	})

	name, ok := idx.lookup("10.0.1.5")
	if !ok || name != "node-a" {
		t.Fatalf("lookup InternalIP = %q, %v", name, ok)
	}
	name, ok = idx.lookup("203.0.113.10")
	if !ok || name != "node-a" {
		t.Fatalf("lookup ExternalIP = %q, %v", name, ok)
	}
	if _, ok := idx.lookup("node-a"); ok {
		t.Fatal("hostname must not be indexed")
	}
	if _, ok := idx.lookup("10.0.0.1"); ok {
		t.Fatal("unexpected IP match")
	}
}

func TestNodeIPIndexAmbiguous(t *testing.T) {
	idx := buildNodeIPIndex([]*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "node-a"},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.0.1.5"}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "node-b"},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.0.1.5"}},
			},
		},
	})

	if _, ok := idx.lookup("10.0.1.5"); ok {
		t.Fatal("ambiguous node IP must not match")
	}
}

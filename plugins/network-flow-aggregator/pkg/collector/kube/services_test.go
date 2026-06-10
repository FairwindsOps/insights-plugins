package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestServiceIndexLookup(t *testing.T) {
	idx := buildServiceIndex([]*corev1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "postgres"},
			Spec: corev1.ServiceSpec{
				ClusterIP: "10.96.0.10",
				Ports:     []corev1.ServicePort{{Port: 5432}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "headless"},
			Spec: corev1.ServiceSpec{
				ClusterIP: corev1.ClusterIPNone,
				Ports:     []corev1.ServicePort{{Port: 80}},
			},
		},
	})

	ref, ok := idx.lookup("10.96.0.10", 5432)
	if !ok {
		t.Fatal("expected match")
	}
	if ref.Namespace != "prod" || ref.Name != "postgres" {
		t.Fatalf("ref = %#v", ref)
	}

	if _, ok := idx.lookup("10.96.0.10", 80); ok {
		t.Fatal("unexpected port match")
	}
	if _, ok := idx.lookup("10.0.0.1", 5432); ok {
		t.Fatal("unexpected ip match")
	}
}

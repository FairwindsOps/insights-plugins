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

func TestClusterIPIndexLookup(t *testing.T) {
	idx := buildClusterIPIndex([]*corev1.Service{
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

	ref, ok := idx.lookup("10.96.0.10")
	if !ok {
		t.Fatal("expected ClusterIP match")
	}
	if ref.Namespace != "prod" || ref.Name != "postgres" {
		t.Fatalf("ref = %#v", ref)
	}
	if _, ok := idx.lookup("None"); ok {
		t.Fatal("headless must not be indexed")
	}
}

func TestClusterIPIndexExtraAddrs(t *testing.T) {
	idx := buildClusterIPIndex([]*corev1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "api"},
			Spec: corev1.ServiceSpec{
				ClusterIP:    "10.96.0.10",
				ClusterIPs:   []string{"10.96.0.10", "fd00::10"},
				ExternalIPs:  []string{"203.0.113.10"},
				Ports:        []corev1.ServicePort{{Port: 443}},
			},
			Status: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{{IP: "198.51.100.20"}},
				},
			},
		},
	})

	for _, addr := range []string{"10.96.0.10", "fd00::10", "203.0.113.10", "198.51.100.20"} {
		ref, ok := idx.lookup(addr)
		if !ok || ref.Name != "api" {
			t.Fatalf("lookup(%s) = %#v, ok=%v, want api", addr, ref, ok)
		}
	}
}

func TestClusterIPIndexAmbiguous(t *testing.T) {
	idx := buildClusterIPIndex([]*corev1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "a", Name: "svc-a"},
			Spec:       corev1.ServiceSpec{ClusterIP: "10.96.0.50", Ports: []corev1.ServicePort{{Port: 80}}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "b", Name: "svc-b"},
			Spec:       corev1.ServiceSpec{ClusterIP: "10.96.0.50", Ports: []corev1.ServicePort{{Port: 80}}},
		},
	})

	if _, ok := idx.lookup("10.96.0.50"); ok {
		t.Fatal("ambiguous ClusterIP must not match")
	}
}

func TestServiceIndexIndexesExternalAndLB(t *testing.T) {
	idx := buildServiceIndex([]*corev1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "web"},
			Spec: corev1.ServiceSpec{
				ClusterIP:   "10.96.0.1",
				ExternalIPs: []string{"203.0.113.5"},
				Ports:       []corev1.ServicePort{{Port: 80}},
			},
			Status: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{{IP: "198.51.100.5"}},
				},
			},
		},
	})

	for _, addr := range []string{"10.96.0.1", "203.0.113.5", "198.51.100.5"} {
		ref, ok := idx.lookup(addr, 80)
		if !ok || ref.Name != "web" {
			t.Fatalf("lookup(%s, 80) = %#v, ok=%v", addr, ref, ok)
		}
	}
}

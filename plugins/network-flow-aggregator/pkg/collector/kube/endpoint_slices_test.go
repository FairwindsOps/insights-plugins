package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestDstIndexClusterIPAndPodIP(t *testing.T) {
	services := []*corev1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "insights", Name: "demo-server"},
			Spec: corev1.ServiceSpec{
				ClusterIP: "10.96.173.46",
				Ports:     []corev1.ServicePort{{Port: 8080, TargetPort: intstr.FromInt32(8080)}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "web"},
			Spec: corev1.ServiceSpec{
				ClusterIP: "10.96.0.20",
				Ports:     []corev1.ServicePort{{Port: 80, TargetPort: intstr.FromInt32(8080)}},
			},
		},
	}

	ready := true
	slices := []*discoveryv1.EndpointSlice{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "insights",
				Name:      "demo-server-abc",
				Labels:    map[string]string{discoveryv1.LabelServiceName: "demo-server"},
			},
			Ports: []discoveryv1.EndpointPort{{Port: ptrInt32(8080)}},
			Endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{"10.244.0.15"}, Conditions: discoveryv1.EndpointConditions{Ready: &ready}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "prod",
				Name:      "web-abc",
				Labels:    map[string]string{discoveryv1.LabelServiceName: "web"},
			},
			Ports: []discoveryv1.EndpointPort{{Port: ptrInt32(80)}},
			Endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{"10.244.1.10"}, Conditions: discoveryv1.EndpointConditions{Ready: &ready}},
			},
		},
	}

	idx := buildDstIndex(services, slices)

	ref, ok := idx.lookup("10.96.173.46", 8080)
	if !ok || ref.Name != "demo-server" {
		t.Fatalf("clusterIP lookup = %#v, ok=%v", ref, ok)
	}

	ref, ok = idx.lookup("10.244.0.15", 8080)
	if !ok || ref.Namespace != "insights" || ref.Name != "demo-server" {
		t.Fatalf("pod IP lookup = %#v, ok=%v", ref, ok)
	}

	ref, ok = idx.lookup("10.244.1.10", 8080)
	if !ok || ref.Name != "web" {
		t.Fatalf("targetPort lookup = %#v, ok=%v", ref, ok)
	}
}

func TestEndpointSliceSkipsNotReady(t *testing.T) {
	services := []*corev1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "api"},
			Spec: corev1.ServiceSpec{
				ClusterIP: "10.96.0.30",
				Ports:     []corev1.ServicePort{{Port: 443}},
			},
		},
	}

	notReady := false
	slices := []*discoveryv1.EndpointSlice{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "api-abc",
				Labels:    map[string]string{discoveryv1.LabelServiceName: "api"},
			},
			Ports: []discoveryv1.EndpointPort{{Port: ptrInt32(443)}},
			Endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{"10.244.2.5"}, Conditions: discoveryv1.EndpointConditions{Ready: &notReady}},
			},
		},
	}

	idx := buildDstIndex(services, slices)
	if _, ok := idx.lookup("10.244.2.5", 443); ok {
		t.Fatal("expected not-ready endpoint to be skipped")
	}
	if _, ok := idx.lookup("10.96.0.30", 443); !ok {
		t.Fatal("expected clusterIP lookup to still work")
	}
}

func TestEndpointIndexPodTargetRef(t *testing.T) {
	services := []*corev1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "payments", Name: "backend"},
			Spec: corev1.ServiceSpec{
				ClusterIP: "10.96.89.41",
				Ports:     []corev1.ServicePort{{Port: 8080, TargetPort: intstr.FromInt32(8080)}},
			},
		},
	}

	ready := true
	slices := []*discoveryv1.EndpointSlice{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "payments",
				Name:      "backend-abc",
				Labels:    map[string]string{discoveryv1.LabelServiceName: "backend"},
			},
			Ports: []discoveryv1.EndpointPort{{Port: ptrInt32(8080)}},
			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: []string{"10.244.0.95"},
					Conditions: discoveryv1.EndpointConditions{Ready: &ready},
					TargetRef: &corev1.ObjectReference{
						Kind:      "Pod",
						Namespace: "payments",
						Name:      "backend-6f9c48f647-vnbzk",
					},
				},
			},
		},
	}

	idx := buildEndpointIndex(services, slices)
	entry, ok := idx.lookup("10.244.0.95", 8080)
	if !ok {
		t.Fatal("expected endpoint lookup")
	}
	if entry.ServiceName != "backend" || entry.PodName != "backend-6f9c48f647-vnbzk" {
		t.Fatalf("entry = %#v", entry)
	}
}

func TestEndpointIPIndexUnique(t *testing.T) {
	ready := true
	idx := buildEndpointIPIndex([]*discoveryv1.EndpointSlice{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "kubernetes",
				Labels:    map[string]string{discoveryv1.LabelServiceName: "kubernetes"},
			},
			Ports: []discoveryv1.EndpointPort{{Port: ptrInt32(443)}},
			Endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{"172.20.25.214", "172.20.26.120"}, Conditions: discoveryv1.EndpointConditions{Ready: &ready}},
			},
		},
	})

	ref, ok := idx.lookup("172.20.25.214")
	if !ok || ref.Namespace != "default" || ref.Name != "kubernetes" {
		t.Fatalf("lookup = %#v, ok=%v", ref, ok)
	}
	ref, ok = idx.lookup("172.20.26.120")
	if !ok || ref.Name != "kubernetes" {
		t.Fatalf("lookup second IP = %#v, ok=%v", ref, ok)
	}
}

func TestEndpointIPIndexAmbiguous(t *testing.T) {
	ready := true
	idx := buildEndpointIPIndex([]*discoveryv1.EndpointSlice{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "a",
				Name:      "svc-a",
				Labels:    map[string]string{discoveryv1.LabelServiceName: "svc-a"},
			},
			Endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{"10.244.0.1"}, Conditions: discoveryv1.EndpointConditions{Ready: &ready}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "b",
				Name:      "svc-b",
				Labels:    map[string]string{discoveryv1.LabelServiceName: "svc-b"},
			},
			Endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{"10.244.0.1"}, Conditions: discoveryv1.EndpointConditions{Ready: &ready}},
			},
		},
	})

	if _, ok := idx.lookup("10.244.0.1"); ok {
		t.Fatal("ambiguous endpoint IP must not match")
	}
}

func TestEndpointIPIndexSkipsNotReadyAndMissingLabel(t *testing.T) {
	ready := true
	notReady := false
	idx := buildEndpointIPIndex([]*discoveryv1.EndpointSlice{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "api-not-ready",
				Labels:    map[string]string{discoveryv1.LabelServiceName: "api"},
			},
			Endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{"10.244.2.5"}, Conditions: discoveryv1.EndpointConditions{Ready: &notReady}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "orphan",
			},
			Endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{"10.244.2.6"}, Conditions: discoveryv1.EndpointConditions{Ready: &ready}},
			},
		},
	})

	if _, ok := idx.lookup("10.244.2.5"); ok {
		t.Fatal("not-ready endpoint must be skipped")
	}
	if _, ok := idx.lookup("10.244.2.6"); ok {
		t.Fatal("missing service-name label must be skipped")
	}
}

func ptrInt32(v int32) *int32 {
	return &v
}

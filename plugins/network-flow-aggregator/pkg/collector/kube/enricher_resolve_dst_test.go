package kube

import (
	"context"
	"log/slog"
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	corelisters "k8s.io/client-go/listers/core/v1"
	discoverylisters "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/client-go/tools/cache"

	ctrlclient "github.com/fairwindsops/controller-utils/pkg/controller"
)

func newTestEnricher(t *testing.T, pods []*corev1.Pod, services []*corev1.Service, slices []*discoveryv1.EndpointSlice) *Enricher {
	t.Helper()

	podIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, pod := range pods {
		if err := podIndexer.Add(pod); err != nil {
			t.Fatalf("add pod: %v", err)
		}
	}

	svcIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, svc := range services {
		if err := svcIndexer.Add(svc); err != nil {
			t.Fatalf("add service: %v", err)
		}
	}

	sliceIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, slice := range slices {
		if err := sliceIndexer.Add(slice); err != nil {
			t.Fatalf("add endpointslice: %v", err)
		}
	}

	return &Enricher{
		log:           slog.Default(),
		controller:    ctrlclient.Client{Context: context.Background()},
		podLister:     corelisters.NewPodLister(podIndexer),
		svcLister:     corelisters.NewServiceLister(svcIndexer),
		epSliceLister: discoverylisters.NewEndpointSliceLister(sliceIndexer),
		ownerCache:    make(map[string]unstructured.Unstructured),
	}
}

func TestResolveDstPrefersServiceOverPodIP(t *testing.T) {
	ready := true
	e := newTestEnricher(t,
		[]*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: "insights", Name: "demo-server-abc"},
				Status:     corev1.PodStatus{PodIP: "10.244.0.15", Phase: corev1.PodRunning},
			},
		},
		[]*corev1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: "insights", Name: "demo-server"},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.96.173.46",
					Ports:     []corev1.ServicePort{{Port: 8080}},
				},
			},
		},
		[]*discoveryv1.EndpointSlice{
			{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "insights",
					Name:      "demo-server-slice",
					Labels:    map[string]string{discoveryv1.LabelServiceName: "demo-server"},
				},
				Ports: []discoveryv1.EndpointPort{{Port: ptrInt32(8080)}},
				Endpoints: []discoveryv1.Endpoint{
					{Addresses: []string{"10.244.0.15"}, Conditions: discoveryv1.EndpointConditions{Ready: &ready}},
				},
			},
		},
	)

	dst := e.ResolveDst("10.244.0.15", 8080)
	if dst.Kind != "Service" || dst.Name != "demo-server" {
		t.Fatalf("dst = %#v, want Service demo-server", dst)
	}
}

func TestResolveDstPodIPEphemeralPort(t *testing.T) {
	e := newTestEnricher(t,
		[]*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: "fwinsights-be-main", Name: "fwinsights-api-abc"},
				Status:     corev1.PodStatus{PodIP: "172.20.108.47", Phase: corev1.PodRunning},
			},
		},
		nil,
		nil,
	)

	dst := e.ResolveDst("172.20.108.47", 52912)
	if dst.Kind != "Pod" || dst.Name != "fwinsights-api-abc" || dst.Namespace != "fwinsights-be-main" {
		t.Fatalf("dst = %#v, want Pod fwinsights-api-abc in fwinsights-be-main", dst)
	}
}

func TestResolveDstUnknownPodIP(t *testing.T) {
	e := newTestEnricher(t, nil, nil, nil)

	dst := e.ResolveDst("172.20.108.47", 52912)
	if dst.Kind != "" || dst.Name != "" {
		t.Fatalf("dst = %#v, want empty identity", dst)
	}
	if dst.Addr != "172.20.108.47" {
		t.Fatalf("addr = %q", dst.Addr)
	}
}

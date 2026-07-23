package kube

import (
	"log/slog"
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	discoverylisters "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/client-go/tools/cache"
)

func newTestEnricher(t *testing.T, pods []*corev1.Pod, services []*corev1.Service, slices []*discoveryv1.EndpointSlice, nodes ...*corev1.Node) *Enricher {
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

	nodeIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, node := range nodes {
		if err := nodeIndexer.Add(node); err != nil {
			t.Fatalf("add node: %v", err)
		}
	}

	return &Enricher{
		log:           slog.Default(),
		podLister:     corelisters.NewPodLister(podIndexer),
		svcLister:     corelisters.NewServiceLister(svcIndexer),
		epSliceLister: discoverylisters.NewEndpointSliceLister(sliceIndexer),
		nodeLister:    corelisters.NewNodeLister(nodeIndexer),
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

func TestResolveDstNodeIP(t *testing.T) {
	e := newTestEnricher(t, nil, nil, nil,
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "ip-10-0-1-50.ec2.internal"},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: "172.20.53.225"},
					{Type: corev1.NodeExternalIP, Address: "54.1.2.3"},
				},
			},
		},
	)

	dst := e.ResolveDst("172.20.53.225", 52912)
	if dst.Kind != "Node" || dst.Name != "ip-10-0-1-50.ec2.internal" {
		t.Fatalf("dst = %#v, want Node ip-10-0-1-50.ec2.internal", dst)
	}

	dst = e.ResolveDst("54.1.2.3", 443)
	if dst.Kind != "Node" || dst.Name != "ip-10-0-1-50.ec2.internal" {
		t.Fatalf("dst = %#v, want Node via ExternalIP", dst)
	}
}

func TestResolveDstPrefersServiceOverNodeIP(t *testing.T) {
	ready := true
	e := newTestEnricher(t,
		nil,
		[]*corev1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "kubelet"},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.96.0.99",
					Ports:     []corev1.ServicePort{{Port: 10250}},
				},
			},
		},
		[]*discoveryv1.EndpointSlice{
			{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "kube-system",
					Name:      "kubelet-slice",
					Labels:    map[string]string{discoveryv1.LabelServiceName: "kubelet"},
				},
				Ports: []discoveryv1.EndpointPort{{Port: ptrInt32(10250)}},
				Endpoints: []discoveryv1.Endpoint{
					{Addresses: []string{"172.20.53.225"}, Conditions: discoveryv1.EndpointConditions{Ready: &ready}},
				},
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-a"},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "172.20.53.225"}},
			},
		},
	)

	dst := e.ResolveDst("172.20.53.225", 10250)
	if dst.Kind != "Service" || dst.Name != "kubelet" {
		t.Fatalf("dst = %#v, want Service kubelet", dst)
	}

	// EndpointSlice any-port fallback attributes the unique endpoint IP to the Service.
	dst = e.ResolveDst("172.20.53.225", 52912)
	if dst.Kind != "Service" || dst.Name != "kubelet" {
		t.Fatalf("dst = %#v, want Service kubelet via endpoint IP fallback", dst)
	}
}

func TestResolveDstAmbiguousNodeIP(t *testing.T) {
	e := newTestEnricher(t, nil, nil, nil,
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-a"},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "172.20.1.1"}},
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-b"},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "172.20.1.1"}},
			},
		},
	)

	dst := e.ResolveDst("172.20.1.1", 80)
	if dst.Kind != "" || dst.Name != "" {
		t.Fatalf("dst = %#v, want empty identity for ambiguous node IP", dst)
	}
}

func TestResolveDstSkipsHostNetworkPodForNode(t *testing.T) {
	e := newTestEnricher(t,
		[]*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "aws-node-xyz"},
				Spec:       corev1.PodSpec{HostNetwork: true},
				Status:     corev1.PodStatus{PodIP: "172.20.53.225", Phase: corev1.PodRunning},
			},
		},
		nil,
		nil,
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-a"},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "172.20.53.225"}},
			},
		},
	)

	dst := e.ResolveDst("172.20.53.225", 8162)
	if dst.Kind != "Node" || dst.Name != "node-a" {
		t.Fatalf("dst = %#v, want Node (not hostNetwork DaemonSet)", dst)
	}
}

func TestResolveDstLoopback(t *testing.T) {
	e := newTestEnricher(t, nil, nil, nil)

	for _, addr := range []string{"127.0.0.1", "::1"} {
		dst := e.ResolveDst(addr, 8080)
		if dst.Kind != "Loopback" || dst.Name != "localhost" || dst.Addr != addr {
			t.Fatalf("dst = %#v for %s, want Loopback localhost", dst, addr)
		}
	}
}

func TestResolveDstClusterIPPortFallback(t *testing.T) {
	e := newTestEnricher(t,
		nil,
		[]*corev1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "postgres"},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.96.0.10",
					Ports:     []corev1.ServicePort{{Port: 5432}},
				},
			},
		},
		nil,
	)

	dst := e.ResolveDst("10.96.0.10", 5432)
	if dst.Kind != "Service" || dst.Name != "postgres" {
		t.Fatalf("dst = %#v, want Service on exact port", dst)
	}

	dst = e.ResolveDst("10.96.0.10", 9999)
	if dst.Kind != "Service" || dst.Name != "postgres" || dst.Namespace != "prod" {
		t.Fatalf("dst = %#v, want Service via ClusterIP fallback", dst)
	}
}

func TestResolveDstClusterIPFallbackBeforeNode(t *testing.T) {
	e := newTestEnricher(t,
		nil,
		[]*corev1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "api"},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.96.0.20",
					Ports:     []corev1.ServicePort{{Port: 80}},
				},
			},
		},
		nil,
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-a"},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.96.0.20"}},
			},
		},
	)

	dst := e.ResolveDst("10.96.0.20", 9999)
	if dst.Kind != "Service" || dst.Name != "api" {
		t.Fatalf("dst = %#v, want Service via ClusterIP before Node", dst)
	}
}

func TestResolveDstEndpointIPAnyPort(t *testing.T) {
	ready := true
	e := newTestEnricher(t,
		nil,
		[]*corev1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "kubernetes"},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.96.0.1",
					Ports:     []corev1.ServicePort{{Port: 443}},
				},
			},
		},
		[]*discoveryv1.EndpointSlice{
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
		},
	)

	dst := e.ResolveDst("172.20.25.214", 443)
	if dst.Kind != "Service" || dst.Name != "kubernetes" || dst.Namespace != "default" {
		t.Fatalf("dst = %#v, want Service on exact port", dst)
	}

	dst = e.ResolveDst("172.20.25.214", 6443)
	if dst.Kind != "Service" || dst.Name != "kubernetes" || dst.Namespace != "default" {
		t.Fatalf("dst = %#v, want Service via endpoint IP any-port", dst)
	}

	dst = e.ResolveDst("172.20.26.120", 80)
	if dst.Kind != "Service" || dst.Name != "kubernetes" {
		t.Fatalf("dst = %#v, want Service for second endpoint IP", dst)
	}
}

func TestResolveDstEndpointIPAnyPortBeforePod(t *testing.T) {
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

	dst := e.ResolveDst("10.244.0.15", 9999)
	if dst.Kind != "Service" || dst.Name != "demo-server" {
		t.Fatalf("dst = %#v, want Service via endpoint IP before Pod", dst)
	}
}

func TestResolveDstAmbiguousEndpointIP(t *testing.T) {
	ready := true
	e := newTestEnricher(t,
		[]*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: "insights", Name: "shared-pod"},
				Status:     corev1.PodStatus{PodIP: "10.244.0.1", Phase: corev1.PodRunning},
			},
		},
		nil,
		[]*discoveryv1.EndpointSlice{
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
		},
	)

	dst := e.ResolveDst("10.244.0.1", 9999)
	if dst.Kind != "Pod" || dst.Name != "shared-pod" {
		t.Fatalf("dst = %#v, want Pod after ambiguous endpoint IP miss", dst)
	}
}

func TestResolveDstLinkLocal(t *testing.T) {
	e := newTestEnricher(t, nil, nil, nil)

	dst := e.ResolveDst("169.254.169.254", 80)
	if dst.Kind != "LinkLocal" || dst.Name != "metadata" || dst.Addr != "169.254.169.254" {
		t.Fatalf("dst = %#v, want LinkLocal metadata", dst)
	}

	dst = e.ResolveDst("fe80::1", 80)
	if dst.Kind != "LinkLocal" || dst.Name != "metadata" {
		t.Fatalf("dst = %#v, want LinkLocal for IPv6", dst)
	}
}

func TestResolveDstPartialListFailure(t *testing.T) {
	e := newTestEnricher(t,
		nil,
		[]*corev1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "postgres"},
				Spec: corev1.ServiceSpec{
					ClusterIP: "10.96.0.10",
					Ports:     []corev1.ServicePort{{Port: 5432}},
				},
			},
		},
		nil,
	)
	e.epSliceLister = nil // EndpointSlice list unavailable

	dst := e.ResolveDst("10.96.0.10", 9999)
	if dst.Kind != "Service" || dst.Name != "postgres" {
		t.Fatalf("dst = %#v, want ClusterIP resolve when EndpointSlice list fails", dst)
	}
}

func TestResolveDstBothListsFailStillResolvesLoopbackAndLinkLocal(t *testing.T) {
	e := newTestEnricher(t, nil, nil, nil)
	e.svcLister = nil
	e.epSliceLister = nil

	dst := e.ResolveDst("127.0.0.1", 8080)
	if dst.Kind != "Loopback" || dst.Name != "localhost" {
		t.Fatalf("dst = %#v, want Loopback when service lists fail", dst)
	}

	dst = e.ResolveDst("169.254.169.254", 80)
	if dst.Kind != "LinkLocal" || dst.Name != "metadata" {
		t.Fatalf("dst = %#v, want LinkLocal when service lists fail", dst)
	}
}

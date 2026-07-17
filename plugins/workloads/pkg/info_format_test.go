package workloads

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"
)

func TestFormatNodeConditions(t *testing.T) {
	require.Nil(t, formatNodeConditions(nil))
	require.Nil(t, formatNodeConditions([]corev1.NodeCondition{}))

	ts := metav1.NewTime(time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC))
	got := formatNodeConditions([]corev1.NodeCondition{{
		Type:               corev1.NodeReady,
		Status:             corev1.ConditionTrue,
		Reason:             "KubeletReady",
		Message:            "ok",
		LastTransitionTime: ts,
	}})
	require.Equal(t, []NodeConditionSummary{{
		Type:               "Ready",
		Status:             "True",
		Reason:             "KubeletReady",
		Message:            "ok",
		LastTransitionTime: ts.UTC(),
	}}, got)
}

func TestFormatNodeTaints(t *testing.T) {
	require.Nil(t, formatNodeTaints(nil))
	got := formatNodeTaints([]corev1.Taint{{
		Key:    "node-role.kubernetes.io/control-plane",
		Effect: corev1.TaintEffectNoSchedule,
		Value:  "true",
	}})
	require.Equal(t, []NodeTaintSummary{{
		Key:    "node-role.kubernetes.io/control-plane",
		Value:  "true",
		Effect: "NoSchedule",
	}}, got)
}

func TestFormatNodeAddresses(t *testing.T) {
	require.Nil(t, formatNodeAddresses(nil))
	got := formatNodeAddresses([]corev1.NodeAddress{
		{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
		{Type: corev1.NodeHostName, Address: "node-1"},
	})
	require.Equal(t, []NodeAddressSummary{
		{Type: "InternalIP", Address: "10.0.0.1"},
		{Type: "Hostname", Address: "node-1"},
	}, got)
}

func TestFormatNodeInfo(t *testing.T) {
	got := formatNodeInfo(corev1.NodeSystemInfo{
		Architecture:            "amd64",
		OperatingSystem:         "linux",
		OSImage:                 "Ubuntu 24.04",
		ContainerRuntimeVersion: "containerd://1.7.0",
		KernelVersion:           "6.8.0",
		KubeletVersion:          "v1.32.0",
	})
	require.Equal(t, NodeInfoSummary{
		Architecture:            "amd64",
		OperatingSystem:         "linux",
		OSImage:                 "Ubuntu 24.04",
		ContainerRuntimeVersion: "containerd://1.7.0",
		KernelVersion:           "6.8.0",
		KubeletVersion:          "v1.32.0",
	}, got)
}

func TestServiceBackendPortString(t *testing.T) {
	require.Equal(t, "http", serviceBackendPortString(networkingv1.ServiceBackendPort{Name: "http", Number: 80}))
	require.Equal(t, "443", serviceBackendPortString(networkingv1.ServiceBackendPort{Number: 443}))
	require.Equal(t, "", serviceBackendPortString(networkingv1.ServiceBackendPort{}))
}

func TestFormatIngressBackend(t *testing.T) {
	require.Nil(t, formatIngressBackend(nil))

	svc := formatIngressBackend(&networkingv1.IngressBackend{
		Service: &networkingv1.IngressServiceBackend{
			Name: "web",
			Port: networkingv1.ServiceBackendPort{Number: 80},
		},
	})
	require.Equal(t, &IngressBackendSummary{ServiceName: "web", ServicePort: "80"}, svc)

	apiGroup := "example.com"
	res := formatIngressBackend(&networkingv1.IngressBackend{
		Resource: &corev1.TypedLocalObjectReference{
			APIGroup: &apiGroup,
			Kind:     "ServiceImport",
			Name:     "imported",
		},
	})
	require.Equal(t, &IngressBackendSummary{
		API:  "example.com",
		Kind: "ServiceImport",
		Name: "imported",
	}, res)
}

func TestFormatIngressRules(t *testing.T) {
	require.Nil(t, formatIngressRules(nil))

	pathType := networkingv1.PathTypePrefix
	got := formatIngressRules([]networkingv1.IngressRule{{
		Host: "example.com",
		IngressRuleValue: networkingv1.IngressRuleValue{
			HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{{
					Path:     "/",
					PathType: &pathType,
					Backend: networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "web",
							Port: networkingv1.ServiceBackendPort{Name: "http"},
						},
					},
				}},
			},
		},
	}})
	require.Equal(t, []IngressRuleSummary{{
		Host: "example.com",
		Paths: []IngressPathSummary{{
			Path:     "/",
			PathType: "Prefix",
			Backend: IngressBackendSummary{
				ServiceName: "web",
				ServicePort: "http",
			},
		}},
	}}, got)
}

func TestFormatIngressTLS(t *testing.T) {
	require.Nil(t, formatIngressTLS(nil))
	got := formatIngressTLS([]networkingv1.IngressTLS{{
		Hosts:      []string{"example.com"},
		SecretName: "tls-secret",
	}})
	require.Equal(t, []IngressTLSSummary{{
		Hosts:      []string{"example.com"},
		SecretName: "tls-secret",
	}}, got)
}

func TestFormatIngressLoadBalancer(t *testing.T) {
	require.Nil(t, formatIngressLoadBalancer(networkingv1.IngressLoadBalancerStatus{}))
	got := formatIngressLoadBalancer(networkingv1.IngressLoadBalancerStatus{
		Ingress: []networkingv1.IngressLoadBalancerIngress{
			{IP: "1.2.3.4"},
			{Hostname: "lb.example.com"},
		},
	})
	require.Equal(t, []IngressLoadBalancerEntry{
		{IP: "1.2.3.4"},
		{Hostname: "lb.example.com"},
	}, got)
}

func TestResolveIngressAPIVersion(t *testing.T) {
	require.Equal(t, networkingIngressAPIVersion, resolveIngressAPIVersion(networkingv1.Ingress{}))
	require.Equal(t, "networking.k8s.io/v1", resolveIngressAPIVersion(networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{APIVersion: "networking.k8s.io/v1"},
	}))
	// ManagedFields are ignored; missing TypeMeta.APIVersion falls back to networking/v1.
	require.Equal(t, networkingIngressAPIVersion, resolveIngressAPIVersion(networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			ManagedFields: []metav1.ManagedFieldsEntry{{
				APIVersion: "networking.k8s.io/v1beta1",
			}},
		},
	}))
}

func TestFormatIngress(t *testing.T) {
	className := "nginx"
	pathType := networkingv1.PathTypeExact
	item := networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{APIVersion: "networking.k8s.io/v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "web",
			Namespace:   "default",
			UID:         types.UID("uid-1"),
			Labels:      map[string]string{"app": "web"},
			Annotations: map[string]string{"a": "b"},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &className,
			DefaultBackend: &networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: "fallback",
					Port: networkingv1.ServiceBackendPort{Number: 8080},
				},
			},
			Rules: []networkingv1.IngressRule{{
				Host: "web.example.com",
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/api",
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "api",
									Port: networkingv1.ServiceBackendPort{Number: 80},
								},
							},
						}},
					},
				},
			}},
			TLS: []networkingv1.IngressTLS{{
				Hosts:      []string{"web.example.com"},
				SecretName: "web-tls",
			}},
		},
		Status: networkingv1.IngressStatus{
			LoadBalancer: networkingv1.IngressLoadBalancerStatus{
				Ingress: []networkingv1.IngressLoadBalancerIngress{{Hostname: "lb.example.com"}},
			},
		},
	}

	got := formatIngress(item)
	require.Equal(t, KindIngress, got.Kind)
	require.Equal(t, "web", got.Name)
	require.Equal(t, "default", got.Namespace)
	require.Equal(t, "uid-1", got.UID)
	require.Equal(t, "networking.k8s.io/v1", got.APIVersion)
	require.Equal(t, ptr.To("nginx"), got.IngressClassName)
	require.Equal(t, &IngressBackendSummary{ServiceName: "fallback", ServicePort: "8080"}, got.DefaultBackend)
	require.Len(t, got.Rules, 1)
	require.Equal(t, "web.example.com", got.Rules[0].Host)
	require.Equal(t, []IngressTLSSummary{{Hosts: []string{"web.example.com"}, SecretName: "web-tls"}}, got.TLS)
	require.Equal(t, []IngressLoadBalancerEntry{{Hostname: "lb.example.com"}}, got.LoadBalancer)
}

func TestFormatService(t *testing.T) {
	item := corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web",
			Namespace: "default",
			UID:       types.UID("svc-1"),
			Labels:    map[string]string{"app": "web"},
		},
		Spec: corev1.ServiceSpec{
			Type:       corev1.ServiceTypeNodePort,
			ClusterIP:  "10.0.0.10",
			ClusterIPs: []string{"10.0.0.10"},
			Selector:   map[string]string{"app": "web"},
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Protocol:   corev1.ProtocolTCP,
				Port:       80,
				TargetPort: intstr.FromString("http"),
				NodePort:   30080,
			}},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{{IP: "1.2.3.4"}},
			},
		},
	}

	got := formatService(item)
	require.Equal(t, KindService, got.Kind)
	require.Equal(t, "web", got.Name)
	require.Equal(t, "default", got.Namespace)
	require.Equal(t, "svc-1", got.UID)
	require.Equal(t, "v1", got.APIVersion)
	require.Equal(t, "NodePort", got.Type)
	require.Equal(t, "10.0.0.10", got.ClusterIP)
	require.Equal(t, []string{"10.0.0.10"}, got.ClusterIPs)
	require.Equal(t, map[string]string{"app": "web"}, got.Selector)
	require.Equal(t, []ServicePortSummary{{
		Name:       "http",
		Protocol:   "TCP",
		Port:       80,
		TargetPort: "http",
		NodePort:   30080,
	}}, got.Ports)
	require.Equal(t, []IngressLoadBalancerEntry{{IP: "1.2.3.4"}}, got.LoadBalancer)
}

func TestFormatPersistentVolumeClaim(t *testing.T) {
	storageClass := "standard"
	volumeMode := corev1.PersistentVolumeFilesystem
	item := corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data",
			Namespace: "default",
			UID:       types.UID("pvc-1"),
			Labels:    map[string]string{"app": "db"},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClass,
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			VolumeMode:       &volumeMode,
			VolumeName:       "pv-1",
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("10Gi"),
			},
		},
	}

	got := formatPersistentVolumeClaim(item)
	require.Equal(t, KindPersistentVolumeClaim, got.Kind)
	require.Equal(t, "data", got.Name)
	require.Equal(t, "default", got.Namespace)
	require.Equal(t, "pvc-1", got.UID)
	require.Equal(t, "v1", got.APIVersion)
	require.Equal(t, ptr.To("standard"), got.StorageClassName)
	require.Equal(t, []string{"ReadWriteOnce"}, got.AccessModes)
	require.Equal(t, "Filesystem", got.VolumeMode)
	require.Equal(t, "pv-1", got.VolumeName)
	require.Equal(t, "10Gi", got.RequestStorage)
	require.Equal(t, "10Gi", got.CapacityStorage)
	require.Equal(t, "Bound", got.Phase)
}

func TestCollectNamespaceCounts(t *testing.T) {
	ctx := context.Background()
	kube := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "default"}},
		&corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Name: "rq", Namespace: "default"}},
		&corev1.LimitRange{ObjectMeta: metav1.ObjectMeta{Name: "lr", Namespace: "default"}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "np", Namespace: "default"}},
	)
	namespaces := []corev1.Namespace{
		{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
	}
	pods := []corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: "default"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "p2", Namespace: "default"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "p3", Namespace: "kube-system"}},
	}
	ingresses := []Ingress{
		{Name: "ing-a", Namespace: "default"},
		{Name: "ing-b", Namespace: "default"},
	}
	services := []Service{
		{Name: "svc", Namespace: "default"},
	}

	got := collectNamespaceCounts(ctx, kube, namespaces, pods, services, ingresses)
	require.Equal(t, []NamespaceCounts{
		{
			Name:               "default",
			ResourceQuotaCount: 1,
			LimitRangeCount:    1,
			NetworkPolicyCount: 1,
			PodCount:           2,
			ServiceCount:       1,
			IngressCount:       2,
		},
		{
			Name:     "kube-system",
			PodCount: 1,
		},
	}, got)
}

func TestCollectNamespaceCountsSoftFail(t *testing.T) {
	ctx := context.Background()
	kube := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "default"}},
	)
	kube.Fake.PrependReactor("list", "networkpolicies", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "networking.k8s.io", Resource: "networkpolicies"}, "", nil)
	})
	kube.Fake.PrependReactor("list", "resourcequotas", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "resourcequotas"}, "", nil)
	})
	kube.Fake.PrependReactor("list", "limitranges", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "limitranges"}, "", nil)
	})

	got := collectNamespaceCounts(ctx, kube,
		[]corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: "default"}}},
		[]corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: "default"}}},
		[]Service{{Name: "svc", Namespace: "default"}},
		[]Ingress{{Name: "ing", Namespace: "default"}},
	)
	require.Equal(t, []NamespaceCounts{{
		Name:         "default",
		PodCount:     1,
		ServiceCount: 1,
		IngressCount: 1,
		// ResourceQuota / LimitRange / NetworkPolicy left at 0 after soft-fail
	}}, got)
}

func TestPodsScheduledOnNode(t *testing.T) {
	pods := []corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "a"}, Spec: corev1.PodSpec{NodeName: "n1"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		{ObjectMeta: metav1.ObjectMeta{Name: "b"}, Spec: corev1.PodSpec{NodeName: "n1"}, Status: corev1.PodStatus{Phase: corev1.PodSucceeded}},
		{ObjectMeta: metav1.ObjectMeta{Name: "c"}, Spec: corev1.PodSpec{NodeName: "n2"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		{ObjectMeta: metav1.ObjectMeta{Name: "d"}, Spec: corev1.PodSpec{NodeName: "n1"}, Status: corev1.PodStatus{Phase: corev1.PodFailed}},
	}
	got := podsScheduledOnNode(pods, "n1")
	require.Len(t, got.Items, 1)
	require.Equal(t, "a", got.Items[0].Name)
}

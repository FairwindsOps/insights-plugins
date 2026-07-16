package workloads

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
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
		KubeProxyVersion:        "v1.32.0",
	})
	require.Equal(t, NodeInfoSummary{
		Architecture:            "amd64",
		OperatingSystem:         "linux",
		OSImage:                 "Ubuntu 24.04",
		ContainerRuntimeVersion: "containerd://1.7.0",
		KernelVersion:           "6.8.0",
		KubeletVersion:          "v1.32.0",
		KubeProxyVersion:        "v1.32.0",
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
	require.Equal(t, "networking.k8s.io/v1beta1", resolveIngressAPIVersion(networkingv1.Ingress{
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

func TestCollectNamespaceCounts(t *testing.T) {
	ctx := context.Background()
	kube := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: "default"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p2", Namespace: "default"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p3", Namespace: "kube-system"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "default"}},
		&corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Name: "rq", Namespace: "default"}},
		&corev1.LimitRange{ObjectMeta: metav1.ObjectMeta{Name: "lr", Namespace: "default"}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "np", Namespace: "default"}},
	)
	namespaces := []corev1.Namespace{
		{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
	}
	ingresses := []Ingress{
		{Name: "ing-a", Namespace: "default"},
		{Name: "ing-b", Namespace: "default"},
	}

	got, err := collectNamespaceCounts(ctx, kube, namespaces, ingresses)
	require.NoError(t, err)
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

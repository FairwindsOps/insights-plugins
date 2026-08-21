package workloads

import (
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestFormatGateway(t *testing.T) {
	item := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "Gateway",
		"metadata": map[string]any{
			"name":      "public",
			"namespace": "default",
			"uid":       "gw-uid",
			"labels":    map[string]any{"app": "edge"},
		},
		"spec": map[string]any{
			"gatewayClassName": "istio",
			"listeners": []any{
				map[string]any{
					"name":     "https",
					"protocol": "HTTPS",
					"port":     int64(443),
					"hostname": "api.example.com",
					"tls": map[string]any{
						"mode": "Terminate",
						"certificateRefs": []any{
							map[string]any{"kind": "Secret", "name": "api-tls"},
						},
					},
				},
			},
		},
		"status": map[string]any{
			"addresses": []any{
				map[string]any{"type": "IPAddress", "value": "203.0.113.10"},
			},
			"conditions": []any{
				map[string]any{"type": "Programmed", "status": "True", "reason": "Programmed"},
			},
		},
	}}

	got := formatGateway(item)
	require.Equal(t, KindGateway, got.Kind)
	require.Equal(t, "public", got.Name)
	require.Equal(t, "default", got.Namespace)
	require.Equal(t, "istio", got.GatewayClassName)
	require.Equal(t, "gateway.networking.k8s.io/v1", got.APIVersion)
	require.Len(t, got.Listeners, 1)
	require.Equal(t, "https", got.Listeners[0].Name)
	require.Equal(t, "HTTPS", got.Listeners[0].Protocol)
	require.Equal(t, int32(443), got.Listeners[0].Port)
	require.Equal(t, "Terminate", got.Listeners[0].TLSMode)
	require.Len(t, got.Listeners[0].CertificateRefs, 1)
	require.Equal(t, "api-tls", got.Listeners[0].CertificateRefs[0].Name)
	require.Len(t, got.Addresses, 1)
	require.Equal(t, "203.0.113.10", got.Addresses[0].Value)
	require.Len(t, got.Conditions, 1)
	require.Equal(t, "Programmed", got.Conditions[0].Type)
}

func TestFormatHTTPRoute(t *testing.T) {
	item := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata": map[string]any{
			"name":      "api",
			"namespace": "default",
			"uid":       "route-uid",
		},
		"spec": map[string]any{
			"hostnames": []any{"api.example.com"},
			"parentRefs": []any{
				map[string]any{
					"name":        "public",
					"sectionName": "https",
				},
			},
			"rules": []any{
				map[string]any{
					"matches": []any{
						map[string]any{
							"path": map[string]any{"type": "PathPrefix", "value": "/v1"},
						},
					},
					"backendRefs": []any{
						map[string]any{
							"name":   "api-svc",
							"port":   int64(80),
							"weight": int64(100),
						},
					},
					"filters": []any{
						map[string]any{
							"type": "ExtensionRef",
							"extensionRef": map[string]any{
								"group": "gateway.kgateway.dev",
								"kind":  "DirectResponse",
								"name":  "maintenance",
							},
						},
					},
				},
			},
		},
	}}

	got := formatHTTPRoute(item)
	require.Equal(t, KindHTTPRoute, got.Kind)
	require.Equal(t, "api", got.Name)
	require.Equal(t, []string{"api.example.com"}, got.Hostnames)
	require.Len(t, got.ParentRefs, 1)
	require.Equal(t, "public", got.ParentRefs[0].Name)
	require.Equal(t, "https", got.ParentRefs[0].SectionName)
	require.Len(t, got.Rules, 1)
	require.Len(t, got.Rules[0].Matches, 1)
	require.Equal(t, "PathPrefix", got.Rules[0].Matches[0].PathType)
	require.Equal(t, "/v1", got.Rules[0].Matches[0].Path)
	require.Len(t, got.Rules[0].BackendRefs, 1)
	require.Equal(t, "api-svc", got.Rules[0].BackendRefs[0].Name)
	require.NotNil(t, got.Rules[0].BackendRefs[0].Port)
	require.Equal(t, int32(80), *got.Rules[0].BackendRefs[0].Port)
	require.Len(t, got.Rules[0].ExtensionRefs, 1)
	require.Equal(t, "DirectResponse", got.Rules[0].ExtensionRefs[0].Kind)
	require.Equal(t, "maintenance", got.Rules[0].ExtensionRefs[0].Name)
}

func TestFormatGatewayClass(t *testing.T) {
	item := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "GatewayClass",
		"metadata":   map[string]any{"name": "kgateway", "uid": "gc-uid"},
		"spec": map[string]any{
			"controllerName": "kgateway.dev/kgateway",
			"parametersRef": map[string]any{
				"group":     "gateway.kgateway.dev",
				"kind":      "GatewayParameters",
				"name":      "default",
				"namespace": "kgateway-system",
			},
		},
	}}

	got := formatGatewayClass(item)
	require.Equal(t, KindGatewayClass, got.Kind)
	require.Equal(t, "kgateway.dev/kgateway", got.ControllerName)
	require.NotNil(t, got.ParametersRef)
	require.Equal(t, "GatewayParameters", got.ParametersRef.Kind)
	require.Equal(t, "kgateway-system", got.ParametersRef.Namespace)
}

func TestIsGatewayAPIAbsent(t *testing.T) {
	require.True(t, isGatewayAPIAbsent(apierrors.NewNotFound(schema.GroupResource{Group: "gateway.networking.k8s.io", Resource: "gateways"}, "")))
	require.False(t, isGatewayAPIAbsent(apierrors.NewForbidden(schema.GroupResource{Group: "gateway.networking.k8s.io", Resource: "gateways"}, "", nil)))
}

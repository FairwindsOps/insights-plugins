package workloads

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestFormatKGatewayResource(t *testing.T) {
	item := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": kGatewayAPIVersion,
		"kind":       "TrafficPolicy",
		"metadata": map[string]any{
			"name":      "api-security",
			"namespace": "default",
			"uid":       "policy-uid",
		},
		"spec": map[string]any{
			"targetRefs": []any{
				map[string]any{"group": "gateway.networking.k8s.io", "kind": "HTTPRoute", "name": "api"},
			},
			"extAuth":   map[string]any{"extensionRef": map[string]any{"name": "auth"}},
			"rateLimit": map[string]any{},
		},
		"status": map[string]any{
			"conditions": []any{map[string]any{"type": "Accepted", "status": "True"}},
		},
	}}

	got := formatKGatewayResource("TrafficPolicy", item)
	require.Equal(t, "api-security", got.Name)
	require.Equal(t, "default", got.Namespace)
	require.Equal(t, []string{"extAuth", "rateLimit"}, got.SpecFields)
	require.Len(t, got.TargetRefs, 1)
	require.Equal(t, "HTTPRoute", got.TargetRefs[0].Kind)
	require.Len(t, got.Conditions, 1)
}

func TestFormatKGatewayTargetSelectorsOnly(t *testing.T) {
	item := unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "global-transform"},
		"spec": map[string]any{
			"targetSelectors": []any{
				map[string]any{
					"group": "gateway.networking.k8s.io",
					"kind":  "HTTPRoute",
					"matchLabels": map[string]any{
						"global-policy": "transformation",
					},
				},
			},
			"transformation": map[string]any{},
		},
	}}

	got := formatKGatewayResource("TrafficPolicy", item)
	require.Empty(t, got.TargetRefs)
	require.Len(t, got.TargetSelectors, 1)
	require.Equal(t, "HTTPRoute", got.TargetSelectors[0].Kind)
	require.Equal(t, "transformation", got.TargetSelectors[0].MatchLabels["global-policy"])
	require.Equal(t, []string{"transformation"}, got.SpecFields)
}

func TestFormatKGatewayAncestorConditions(t *testing.T) {
	item := unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "api-security"},
		"status": map[string]any{
			"ancestors": []any{
				map[string]any{
					"conditions": []any{
						map[string]any{"type": "Accepted", "status": "True"},
					},
				},
			},
		},
	}}

	got := formatKGatewayResource("TrafficPolicy", item)
	require.Len(t, got.Conditions, 1)
	require.Equal(t, "Accepted", got.Conditions[0].Type)
}

func TestFormatKGatewayStripsLastAppliedAnnotation(t *testing.T) {
	item := unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name": "api-security",
			"annotations": map[string]any{
				lastAppliedAnnotation: `{"spec":{"body":"secret"}}`,
				"app":                 "edge",
			},
		},
	}}

	got := formatKGatewayResource("TrafficPolicy", item)
	require.Equal(t, "edge", got.Annotations["app"])
	require.NotContains(t, got.Annotations, lastAppliedAnnotation)
}

func TestFormatKGatewaySpecializedFields(t *testing.T) {
	directResponse := unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "maintenance"},
		"spec":     map[string]any{"status": int64(503), "body": "not collected"},
	}}
	got := formatKGatewayResource("DirectResponse", directResponse)
	require.Equal(t, int32(503), got.StatusCode)
	require.Equal(t, []string{"body", "status"}, got.SpecFields)

	backend := unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "external"},
		"spec":     map[string]any{"dynamicForwardProxy": map[string]any{}},
	}}
	require.Equal(t, "dynamicForwardProxy", formatKGatewayResource("Backend", backend).Type)
}

func TestListKGatewayInventory(t *testing.T) {
	backend := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": kGatewayAPIVersion,
		"kind":       "Backend",
		"metadata":   map[string]any{"name": "external", "namespace": "default"},
		"spec":       map[string]any{"static": map[string]any{}},
	}}
	client := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), kGatewayListKinds(), backend)

	got := listKGatewayInventory(context.Background(), client)
	require.NotNil(t, got)
	require.Len(t, got.Backends, 1)
	require.Equal(t, "static", got.Backends[0].Type)
	require.NotNil(t, got.BackendConfigPolicies)
	require.NotNil(t, got.ListenerPolicies)
	require.NotNil(t, got.TrafficPolicies)
}

func TestListKGatewayInventoryAbsent(t *testing.T) {
	client := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), kGatewayListKinds())
	client.PrependReactor("list", "*", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(action.GetResource().GroupResource(), "")
	})

	require.Nil(t, listKGatewayInventory(context.Background(), client))
}

func TestListKGatewayInventoryForbidden(t *testing.T) {
	client := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), kGatewayListKinds())
	client.PrependReactor("list", "*", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(action.GetResource().GroupResource(), "", nil)
	})

	got := listKGatewayInventory(context.Background(), client)
	require.NotNil(t, got)
	require.NotNil(t, got.Backends)
	require.NotNil(t, got.TrafficPolicies)
}

func kGatewayListKinds() map[schema.GroupVersionResource]string {
	out := make(map[schema.GroupVersionResource]string, len(kGatewayResources))
	for _, descriptor := range kGatewayResources {
		out[schema.GroupVersionResource{
			Group: kGatewayAPIGroup, Version: "v1alpha1", Resource: descriptor.resource,
		}] = descriptor.kind + "List"
	}
	return out
}

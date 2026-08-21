package workloads

import (
	"context"
	"sort"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const (
	kGatewayAPIGroup   = "gateway.kgateway.dev"
	kGatewayAPIVersion = "gateway.kgateway.dev/v1alpha1"
)

var kGatewayResources = []struct {
	kind     string
	resource string
}{
	{kind: "Backend", resource: "backends"},
	{kind: "BackendConfigPolicy", resource: "backendconfigpolicies"},
	{kind: "DirectResponse", resource: "directresponses"},
	{kind: "GatewayExtension", resource: "gatewayextensions"},
	{kind: "GatewayParameters", resource: "gatewayparameters"},
	{kind: "HTTPListenerPolicy", resource: "httplistenerpolicies"},
	{kind: "ListenerPolicy", resource: "listenerpolicies"},
	{kind: "TrafficPolicy", resource: "trafficpolicies"},
}

// KGatewayResource is a safe inventory summary shared by kgateway CRDs.
type KGatewayResource struct {
	Kind        string
	Name        string
	Namespace   string
	Annotations map[string]string
	Labels      map[string]string
	UID         string
	APIVersion  string
	TargetRefs  []GatewayObjectRef `json:",omitempty"`
	SpecFields  []string           `json:",omitempty"`
	Type        string             `json:",omitempty"`
	StatusCode  int32              `json:",omitempty"`
	Conditions  []GatewayCondition `json:",omitempty"`
}

// KGateway is optional kgateway inventory nested under GatewayAPI.
type KGateway struct {
	Backends              []KGatewayResource
	BackendConfigPolicies []KGatewayResource
	DirectResponses       []KGatewayResource
	GatewayExtensions     []KGatewayResource
	GatewayParameters     []KGatewayResource
	HTTPListenerPolicies  []KGatewayResource
	ListenerPolicies      []KGatewayResource
	TrafficPolicies       []KGatewayResource
}

func listKGatewayInventory(ctx context.Context, dynamicClient dynamic.Interface) *KGateway {
	result := &KGateway{}
	present := false
	for _, descriptor := range kGatewayResources {
		gvr := schema.GroupVersionResource{Group: kGatewayAPIGroup, Version: "v1alpha1", Resource: descriptor.resource}
		items, err := listNamespacedUnstructured(ctx, dynamicClient, gvr)
		if err != nil {
			if isGatewayAPIAbsent(err) {
				continue
			}
			present = true
			logrus.Warnf("error listing kgateway %s, continuing with empty array: %v", descriptor.kind, err)
			items = []unstructured.Unstructured{}
		} else {
			present = true
		}

		resources := make([]KGatewayResource, 0, len(items))
		for _, item := range items {
			resources = append(resources, formatKGatewayResource(descriptor.kind, item))
		}
		setKGatewayResources(result, descriptor.kind, resources)
	}
	if !present {
		return nil
	}
	ensureKGatewayArrays(result)
	return result
}

func setKGatewayResources(inventory *KGateway, kind string, resources []KGatewayResource) {
	switch kind {
	case "Backend":
		inventory.Backends = resources
	case "BackendConfigPolicy":
		inventory.BackendConfigPolicies = resources
	case "DirectResponse":
		inventory.DirectResponses = resources
	case "GatewayExtension":
		inventory.GatewayExtensions = resources
	case "GatewayParameters":
		inventory.GatewayParameters = resources
	case "HTTPListenerPolicy":
		inventory.HTTPListenerPolicies = resources
	case "ListenerPolicy":
		inventory.ListenerPolicies = resources
	case "TrafficPolicy":
		inventory.TrafficPolicies = resources
	}
}

func ensureKGatewayArrays(inventory *KGateway) {
	if inventory.Backends == nil {
		inventory.Backends = []KGatewayResource{}
	}
	if inventory.BackendConfigPolicies == nil {
		inventory.BackendConfigPolicies = []KGatewayResource{}
	}
	if inventory.DirectResponses == nil {
		inventory.DirectResponses = []KGatewayResource{}
	}
	if inventory.GatewayExtensions == nil {
		inventory.GatewayExtensions = []KGatewayResource{}
	}
	if inventory.GatewayParameters == nil {
		inventory.GatewayParameters = []KGatewayResource{}
	}
	if inventory.HTTPListenerPolicies == nil {
		inventory.HTTPListenerPolicies = []KGatewayResource{}
	}
	if inventory.ListenerPolicies == nil {
		inventory.ListenerPolicies = []KGatewayResource{}
	}
	if inventory.TrafficPolicies == nil {
		inventory.TrafficPolicies = []KGatewayResource{}
	}
}

func formatKGatewayResource(kind string, item unstructured.Unstructured) KGatewayResource {
	spec := nestedMap(item.Object, "spec")
	apiVersion := item.GetAPIVersion()
	if apiVersion == "" {
		apiVersion = kGatewayAPIVersion
	}
	out := KGatewayResource{
		Kind:        kind,
		Name:        item.GetName(),
		Namespace:   item.GetNamespace(),
		Annotations: item.GetAnnotations(),
		Labels:      item.GetLabels(),
		UID:         string(item.GetUID()),
		APIVersion:  apiVersion,
		TargetRefs:  formatGatewayObjectRefs(asAnySlice(spec["targetRefs"])),
		SpecFields:  configuredSpecFields(spec),
		Type:        inferKGatewayType(kind, spec),
		Conditions:  formatGatewayConditions(nestedSlice(item.Object, "status", "conditions")),
	}
	if kind == "DirectResponse" {
		out.StatusCode = asInt32(spec["status"])
	}
	return out
}

func configuredSpecFields(spec map[string]any) []string {
	fields := make([]string, 0, len(spec))
	for key, value := range spec {
		if key == "targetRefs" || key == "targetSelectors" || value == nil {
			continue
		}
		fields = append(fields, key)
	}
	sort.Strings(fields)
	return fields
}

func inferKGatewayType(kind string, spec map[string]any) string {
	if explicit := asString(spec["type"]); explicit != "" {
		return explicit
	}
	var candidates []string
	switch kind {
	case "Backend":
		candidates = []string{"aws", "static", "dynamicForwardProxy", "gcp", "priorityGroups"}
	case "GatewayExtension":
		candidates = []string{"extAuth", "extProc", "rateLimit", "jwt", "oauth2"}
	case "GatewayParameters":
		candidates = []string{"kube", "selfManaged"}
	}
	for _, candidate := range candidates {
		if spec[candidate] != nil {
			return candidate
		}
	}
	return ""
}

func asAnySlice(value any) []any {
	items, _ := value.([]any)
	return items
}

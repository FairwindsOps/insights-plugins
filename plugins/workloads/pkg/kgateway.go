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

type kGatewayDescriptor struct {
	kind     string
	resource string
	slice    func(*KGateway) *[]KGatewayResource
}

var kGatewayResources = []kGatewayDescriptor{
	{"Backend", "backends", func(k *KGateway) *[]KGatewayResource { return &k.Backends }},
	{"BackendConfigPolicy", "backendconfigpolicies", func(k *KGateway) *[]KGatewayResource { return &k.BackendConfigPolicies }},
	{"DirectResponse", "directresponses", func(k *KGateway) *[]KGatewayResource { return &k.DirectResponses }},
	{"GatewayExtension", "gatewayextensions", func(k *KGateway) *[]KGatewayResource { return &k.GatewayExtensions }},
	{"GatewayParameters", "gatewayparameters", func(k *KGateway) *[]KGatewayResource { return &k.GatewayParameters }},
	{"HTTPListenerPolicy", "httplistenerpolicies", func(k *KGateway) *[]KGatewayResource { return &k.HTTPListenerPolicies }},
	{"ListenerPolicy", "listenerpolicies", func(k *KGateway) *[]KGatewayResource { return &k.ListenerPolicies }},
	{"TrafficPolicy", "trafficpolicies", func(k *KGateway) *[]KGatewayResource { return &k.TrafficPolicies }},
}

// GatewayTargetSelector is a label selector used by kgateway policies (targetSelectors).
type GatewayTargetSelector struct {
	Group       string            `json:",omitempty"`
	Kind        string            `json:",omitempty"`
	MatchLabels map[string]string `json:",omitempty"`
}

// KGatewayResource is a safe inventory summary shared by kgateway CRDs.
type KGatewayResource struct {
	Kind            string
	Name            string
	Namespace       string
	Annotations     map[string]string
	Labels          map[string]string
	UID             string
	APIVersion      string
	TargetRefs      []GatewayObjectRef      `json:",omitempty"`
	TargetSelectors []GatewayTargetSelector `json:",omitempty"`
	SpecFields      []string                `json:",omitempty"`
	Type            string                  `json:",omitempty"`
	StatusCode      int32                   `json:",omitempty"`
	Conditions      []GatewayCondition      `json:",omitempty"`
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
		*descriptor.slice(result) = resources
	}
	if !present {
		return nil
	}
	for _, descriptor := range kGatewayResources {
		if *descriptor.slice(result) == nil {
			*descriptor.slice(result) = []KGatewayResource{}
		}
	}
	return result
}

func formatKGatewayResource(kind string, item unstructured.Unstructured) KGatewayResource {
	spec := nestedMap(item.Object, "spec")
	apiVersion := item.GetAPIVersion()
	if apiVersion == "" {
		apiVersion = kGatewayAPIVersion
	}
	out := KGatewayResource{
		Kind:            kind,
		Name:            item.GetName(),
		Namespace:       item.GetNamespace(),
		Annotations:     inventoryAnnotations(item.GetAnnotations()),
		Labels:          item.GetLabels(),
		UID:             string(item.GetUID()),
		APIVersion:      apiVersion,
		TargetRefs:      formatGatewayObjectRefs(asAnySlice(spec["targetRefs"])),
		TargetSelectors: formatGatewayTargetSelectors(asAnySlice(spec["targetSelectors"])),
		SpecFields:      configuredSpecFields(spec),
		Type:            inferKGatewayType(kind, spec),
		Conditions:      formatKGatewayConditions(item.Object),
	}
	if kind == "DirectResponse" {
		out.StatusCode = asInt32(spec["status"])
	}
	return out
}

func formatGatewayTargetSelectors(raw []any) []GatewayTargetSelector {
	if len(raw) == 0 {
		return nil
	}
	out := make([]GatewayTargetSelector, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		sel := GatewayTargetSelector{
			Group:       asString(m["group"]),
			Kind:        asString(m["kind"]),
			MatchLabels: asStringMap(m["matchLabels"]),
		}
		if sel.Group == "" && sel.Kind == "" && len(sel.MatchLabels) == 0 {
			continue
		}
		out = append(out, sel)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func formatKGatewayConditions(obj map[string]any) []GatewayCondition {
	out := formatGatewayConditions(nestedSlice(obj, "status", "conditions"))
	for _, ancestor := range nestedSlice(obj, "status", "ancestors") {
		m, ok := ancestor.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, formatGatewayConditions(asAnySlice(m["conditions"]))...)
	}
	if len(out) == 0 {
		return nil
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

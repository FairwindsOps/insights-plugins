package workloads

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const (
	KindGateway      = "Gateway"
	KindGatewayClass = "GatewayClass"
	KindHTTPRoute    = "HTTPRoute"

	gatewayAPIVersion = "gateway.networking.k8s.io/v1"

	lastAppliedAnnotation = "kubectl.kubernetes.io/last-applied-configuration"
)

var (
	gatewayGVR = schema.GroupVersionResource{
		Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways",
	}
	httpRouteGVR = schema.GroupVersionResource{
		Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes",
	}
	gatewayClassGVR = schema.GroupVersionResource{
		Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gatewayclasses",
	}
)

// GatewayObjectRef is a simplified ParentRef / BackendRef / CertificateRef.
type GatewayObjectRef struct {
	Group       string `json:",omitempty"`
	Kind        string `json:",omitempty"`
	Name        string `json:",omitempty"`
	Namespace   string `json:",omitempty"`
	SectionName string `json:",omitempty"`
	Port        *int32 `json:",omitempty"`
	Weight      *int32 `json:",omitempty"`
}

// GatewayListener is a Gateway listener summary.
type GatewayListener struct {
	Name            string             `json:",omitempty"`
	Protocol        string             `json:",omitempty"`
	Port            int32              `json:",omitempty"`
	Hostname        string             `json:",omitempty"`
	TLSMode         string             `json:",omitempty"`
	CertificateRefs []GatewayObjectRef `json:",omitempty"`
}

// GatewayAddress is a status address entry.
type GatewayAddress struct {
	Type  string `json:",omitempty"`
	Value string `json:",omitempty"`
}

// GatewayCondition is a status condition summary.
type GatewayCondition struct {
	Type               string `json:",omitempty"`
	Status             string `json:",omitempty"`
	Reason             string `json:",omitempty"`
	Message            string `json:",omitempty"`
	LastTransitionTime string `json:",omitempty"`
}

// Gateway is a Gateway API Gateway inventory object (namespaced).
type Gateway struct {
	Kind             string
	Name             string
	Namespace        string
	Annotations      map[string]string
	Labels           map[string]string
	UID              string
	APIVersion       string
	GatewayClassName string             `json:",omitempty"`
	Listeners        []GatewayListener  `json:",omitempty"`
	Addresses        []GatewayAddress   `json:",omitempty"`
	Conditions       []GatewayCondition `json:",omitempty"`
}

// GatewayClass is a cluster-scoped Gateway API class.
type GatewayClass struct {
	Kind           string
	Name           string
	Annotations    map[string]string
	Labels         map[string]string
	UID            string
	APIVersion     string
	ControllerName string             `json:",omitempty"`
	ParametersRef  *GatewayObjectRef  `json:",omitempty"`
	Conditions     []GatewayCondition `json:",omitempty"`
}

// HTTPRouteMatch is a path match summary.
type HTTPRouteMatch struct {
	PathType string `json:",omitempty"`
	Path     string `json:",omitempty"`
}

// HTTPRouteRule is a match + backendRefs rule.
type HTTPRouteRule struct {
	Matches       []HTTPRouteMatch   `json:",omitempty"`
	BackendRefs   []GatewayObjectRef `json:",omitempty"`
	ExtensionRefs []GatewayObjectRef `json:",omitempty"`
}

// HTTPRoute is a Gateway API HTTPRoute inventory object (namespaced).
type HTTPRoute struct {
	Kind        string
	Name        string
	Namespace   string
	Annotations map[string]string
	Labels      map[string]string
	UID         string
	APIVersion  string
	Hostnames   []string           `json:",omitempty"`
	ParentRefs  []GatewayObjectRef `json:",omitempty"`
	Rules       []HTTPRouteRule    `json:",omitempty"`
}

// GatewayAPI is optional Gateway API inventory nested under ClusterWorkloadReport.
// Omitted (nil) when gateway.networking.k8s.io CRDs are not installed.
type GatewayAPI struct {
	Gateways       []Gateway
	GatewayClasses []GatewayClass
	HTTPRoutes     []HTTPRoute
	KGateway       *KGateway `json:",omitempty"`
}

// listGatewayAPIInventory returns nil when Gateway API CRDs are not installed.
// When present, nested arrays are always set (possibly empty). Forbidden list is
// treated as present-but-unreadable (warn + empty array).
//
// kgateway is nested here because it extends Gateway API. If those CRDs are
// absent we skip kgateway lists rather than paying eight 404s on typical clusters.
func listGatewayAPIInventory(ctx context.Context, dynamicClient dynamic.Interface) *GatewayAPI {
	gateways, gatewaysErr := listGateways(ctx, dynamicClient)
	gatewayClasses, classesErr := listGatewayClasses(ctx, dynamicClient)
	httpRoutes, routesErr := listHTTPRoutes(ctx, dynamicClient)

	if gatewaysErr != nil && classesErr != nil && routesErr != nil &&
		isGatewayAPIAbsent(gatewaysErr) && isGatewayAPIAbsent(classesErr) && isGatewayAPIAbsent(routesErr) {
		return nil
	}

	if gatewaysErr != nil {
		logrus.Warnf("error listing Gateways, continuing with empty Gateways: %v", gatewaysErr)
		gateways = []Gateway{}
	}
	if routesErr != nil {
		logrus.Warnf("error listing HTTPRoutes, continuing with empty HTTPRoutes: %v", routesErr)
		httpRoutes = []HTTPRoute{}
	}
	if classesErr != nil {
		logrus.Warnf("error listing GatewayClasses, continuing with empty GatewayClasses: %v", classesErr)
		gatewayClasses = []GatewayClass{}
	}

	return &GatewayAPI{
		Gateways:       gateways,
		GatewayClasses: gatewayClasses,
		HTTPRoutes:     httpRoutes,
		KGateway:       listKGatewayInventory(ctx, dynamicClient),
	}
}

func isGatewayAPIAbsent(err error) bool {
	return apierrors.IsNotFound(err) || meta.IsNoMatchError(err)
}

func listGateways(ctx context.Context, dynamicClient dynamic.Interface) ([]Gateway, error) {
	items, err := listNamespacedUnstructured(ctx, dynamicClient, gatewayGVR)
	if err != nil {
		return nil, err
	}
	out := make([]Gateway, 0, len(items))
	for _, item := range items {
		out = append(out, formatGateway(item))
	}
	return out, nil
}

func listHTTPRoutes(ctx context.Context, dynamicClient dynamic.Interface) ([]HTTPRoute, error) {
	items, err := listNamespacedUnstructured(ctx, dynamicClient, httpRouteGVR)
	if err != nil {
		return nil, err
	}
	out := make([]HTTPRoute, 0, len(items))
	for _, item := range items {
		out = append(out, formatHTTPRoute(item))
	}
	return out, nil
}

func listGatewayClasses(ctx context.Context, dynamicClient dynamic.Interface) ([]GatewayClass, error) {
	items, err := listClusterUnstructured(ctx, dynamicClient, gatewayClassGVR)
	if err != nil {
		return nil, err
	}
	out := make([]GatewayClass, 0, len(items))
	for _, item := range items {
		out = append(out, formatGatewayClass(item))
	}
	return out, nil
}

func listNamespacedUnstructured(
	ctx context.Context,
	dynamicClient dynamic.Interface,
	gvr schema.GroupVersionResource,
) ([]unstructured.Unstructured, error) {
	var items []unstructured.Unstructured
	continueToken := ""
	for {
		list, err := dynamicClient.Resource(gvr).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{Continue: continueToken})
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", gvr.Resource, err)
		}
		items = append(items, list.Items...)
		continueToken = list.GetContinue()
		if continueToken == "" {
			break
		}
	}
	return items, nil
}

func formatGateway(item unstructured.Unstructured) Gateway {
	apiVersion := item.GetAPIVersion()
	if apiVersion == "" {
		apiVersion = gatewayAPIVersion
	}
	return Gateway{
		Kind:             KindGateway,
		Name:             item.GetName(),
		Namespace:        item.GetNamespace(),
		Annotations:      inventoryAnnotations(item.GetAnnotations()),
		Labels:           item.GetLabels(),
		UID:              string(item.GetUID()),
		APIVersion:       apiVersion,
		GatewayClassName: nestedString(item.Object, "spec", "gatewayClassName"),
		Listeners:        formatGatewayListeners(nestedSlice(item.Object, "spec", "listeners")),
		Addresses:        formatGatewayAddresses(nestedSlice(item.Object, "status", "addresses")),
		Conditions:       formatGatewayConditions(nestedSlice(item.Object, "status", "conditions")),
	}
}

func formatHTTPRoute(item unstructured.Unstructured) HTTPRoute {
	apiVersion := item.GetAPIVersion()
	if apiVersion == "" {
		apiVersion = gatewayAPIVersion
	}
	hostnames, _, _ := unstructured.NestedStringSlice(item.Object, "spec", "hostnames")
	if len(hostnames) == 0 {
		hostnames = nil
	}
	return HTTPRoute{
		Kind:        KindHTTPRoute,
		Name:        item.GetName(),
		Namespace:   item.GetNamespace(),
		Annotations: inventoryAnnotations(item.GetAnnotations()),
		Labels:      item.GetLabels(),
		UID:         string(item.GetUID()),
		APIVersion:  apiVersion,
		Hostnames:   hostnames,
		ParentRefs:  formatGatewayObjectRefs(nestedSlice(item.Object, "spec", "parentRefs")),
		Rules:       formatHTTPRouteRules(nestedSlice(item.Object, "spec", "rules")),
	}
}

func formatGatewayClass(item unstructured.Unstructured) GatewayClass {
	apiVersion := item.GetAPIVersion()
	if apiVersion == "" {
		apiVersion = gatewayAPIVersion
	}
	var parametersRef *GatewayObjectRef
	if raw := nestedMap(item.Object, "spec", "parametersRef"); raw != nil {
		refs := formatGatewayObjectRefs([]any{raw})
		if len(refs) > 0 {
			parametersRef = &refs[0]
		}
	}
	return GatewayClass{
		Kind:           KindGatewayClass,
		Name:           item.GetName(),
		Annotations:    inventoryAnnotations(item.GetAnnotations()),
		Labels:         item.GetLabels(),
		UID:            string(item.GetUID()),
		APIVersion:     apiVersion,
		ControllerName: nestedString(item.Object, "spec", "controllerName"),
		ParametersRef:  parametersRef,
		Conditions:     formatGatewayConditions(nestedSlice(item.Object, "status", "conditions")),
	}
}

func formatGatewayListeners(raw []any) []GatewayListener {
	if len(raw) == 0 {
		return nil
	}
	out := make([]GatewayListener, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		listener := GatewayListener{
			Name:     asString(m["name"]),
			Protocol: asString(m["protocol"]),
			Port:     asInt32(m["port"]),
			Hostname: asString(m["hostname"]),
		}
		if tls, ok := m["tls"].(map[string]any); ok {
			listener.TLSMode = asString(tls["mode"])
			if refs, ok := tls["certificateRefs"].([]any); ok {
				listener.CertificateRefs = formatGatewayObjectRefs(refs)
			}
		}
		out = append(out, listener)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func formatGatewayAddresses(raw []any) []GatewayAddress {
	if len(raw) == 0 {
		return nil
	}
	out := make([]GatewayAddress, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		addr := GatewayAddress{
			Type:  asString(m["type"]),
			Value: asString(m["value"]),
		}
		if addr.Type == "" && addr.Value == "" {
			continue
		}
		out = append(out, addr)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func formatGatewayConditions(raw []any) []GatewayCondition {
	if len(raw) == 0 {
		return nil
	}
	out := make([]GatewayCondition, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, GatewayCondition{
			Type:               asString(m["type"]),
			Status:             asString(m["status"]),
			Reason:             asString(m["reason"]),
			Message:            asString(m["message"]),
			LastTransitionTime: asString(m["lastTransitionTime"]),
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func formatGatewayObjectRefs(raw []any) []GatewayObjectRef {
	if len(raw) == 0 {
		return nil
	}
	out := make([]GatewayObjectRef, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		ref := GatewayObjectRef{
			Group:       asString(m["group"]),
			Kind:        asString(m["kind"]),
			Name:        asString(m["name"]),
			Namespace:   asString(m["namespace"]),
			SectionName: asString(m["sectionName"]),
		}
		if port := asInt32Ptr(m["port"]); port != nil {
			ref.Port = port
		}
		if weight := asInt32Ptr(m["weight"]); weight != nil {
			ref.Weight = weight
		}
		if ref.Group == "" && ref.Kind == "" && ref.Name == "" && ref.Namespace == "" && ref.SectionName == "" && ref.Port == nil && ref.Weight == nil {
			continue
		}
		out = append(out, ref)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func formatHTTPRouteRules(raw []any) []HTTPRouteRule {
	if len(raw) == 0 {
		return nil
	}
	out := make([]HTTPRouteRule, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rule := HTTPRouteRule{}
		if matches, ok := m["matches"].([]any); ok {
			rule.Matches = formatHTTPRouteMatches(matches)
		}
		if backends, ok := m["backendRefs"].([]any); ok {
			rule.BackendRefs = formatGatewayObjectRefs(backends)
		}
		if filters, ok := m["filters"].([]any); ok {
			rule.ExtensionRefs = formatHTTPRouteExtensionRefs(filters)
		}
		out = append(out, rule)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func formatHTTPRouteExtensionRefs(raw []any) []GatewayObjectRef {
	refs := make([]any, 0, len(raw))
	for _, item := range raw {
		filter, ok := item.(map[string]any)
		if !ok || asString(filter["type"]) != "ExtensionRef" {
			continue
		}
		if ref, ok := filter["extensionRef"].(map[string]any); ok {
			refs = append(refs, ref)
		}
	}
	return formatGatewayObjectRefs(refs)
}

func formatHTTPRouteMatches(raw []any) []HTTPRouteMatch {
	if len(raw) == 0 {
		return nil
	}
	out := make([]HTTPRouteMatch, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		match := HTTPRouteMatch{}
		if path, ok := m["path"].(map[string]any); ok {
			match.PathType = asString(path["type"])
			match.Path = asString(path["value"])
		}
		if match.PathType == "" && match.Path == "" {
			continue
		}
		out = append(out, match)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func inventoryAnnotations(annos map[string]string) map[string]string {
	if len(annos) == 0 {
		return annos
	}
	if _, ok := annos[lastAppliedAnnotation]; !ok {
		return annos
	}
	out := make(map[string]string, len(annos)-1)
	for k, v := range annos {
		if k == lastAppliedAnnotation {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func asInt32(v any) int32 {
	if v == nil {
		return 0
	}
	switch t := v.(type) {
	case int32:
		return t
	case int64:
		return int32(t)
	case int:
		return int32(t)
	case float64:
		return int32(t)
	default:
		var n int32
		_, _ = fmt.Sscan(asString(v), &n)
		return n
	}
}

func asInt32Ptr(v any) *int32 {
	if v == nil {
		return nil
	}
	n := asInt32(v)
	// Treat missing/zero carefully: port 0 is invalid for Gateway API; omit.
	if n == 0 {
		if asString(v) == "" || asString(v) == "0" {
			// Explicit 0 is still unusual; keep only if key was present with numeric 0.
			switch v.(type) {
			case int, int32, int64, float64:
				return &n
			default:
				return nil
			}
		}
	}
	return &n
}

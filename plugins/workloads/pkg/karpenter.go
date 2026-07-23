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
	KindNodePool     = "NodePool"
	KindNodeClaim    = "NodeClaim"
	KindEC2NodeClass = "EC2NodeClass"

	karpenterNodePoolLabelKey = "karpenter.sh/nodepool"
	karpenterAPIVersion       = "karpenter.sh/v1"
	ec2NodeClassAPIVersion    = "karpenter.k8s.aws/v1"
)

var (
	nodePoolGVR = schema.GroupVersionResource{
		Group: "karpenter.sh", Version: "v1", Resource: "nodepools",
	}
	nodeClaimGVR = schema.GroupVersionResource{
		Group: "karpenter.sh", Version: "v1", Resource: "nodeclaims",
	}
	// AWS-only in this release; Azure/GCP NodeClasses are out of scope.
	ec2NodeClassGVR = schema.GroupVersionResource{
		Group: "karpenter.k8s.aws", Version: "v1", Resource: "ec2nodeclasses",
	}
)

// KarpenterNodeClassRef is a reference from a NodePool/NodeClaim to a NodeClass.
type KarpenterNodeClassRef struct {
	Group string `json:",omitempty"`
	Kind  string `json:",omitempty"`
	Name  string `json:",omitempty"`
}

// KarpenterRequirement mirrors a Karpenter node requirement.
type KarpenterRequirement struct {
	Key      string   `json:",omitempty"`
	Operator string   `json:",omitempty"`
	Values   []string `json:",omitempty"`
}

// KarpenterDisruptionBudget is a NodePool disruption budget summary.
type KarpenterDisruptionBudget struct {
	Nodes    string `json:",omitempty"`
	Schedule string `json:",omitempty"`
	Duration string `json:",omitempty"`
}

// KarpenterDisruption is NodePool disruption configuration.
type KarpenterDisruption struct {
	ConsolidationPolicy string                      `json:",omitempty"`
	ConsolidateAfter    string                      `json:",omitempty"`
	Budgets             []KarpenterDisruptionBudget `json:",omitempty"`
}

// KarpenterCondition is a status condition summary.
type KarpenterCondition struct {
	Type               string `json:",omitempty"`
	Status             string `json:",omitempty"`
	Reason             string `json:",omitempty"`
	Message            string `json:",omitempty"`
	LastTransitionTime string `json:",omitempty"`
}

// KarpenterSelectorTerm is a simplified AMI/subnet/security-group selector term.
type KarpenterSelectorTerm struct {
	ID    string            `json:",omitempty"`
	Name  string            `json:",omitempty"`
	Alias string            `json:",omitempty"`
	Owner string            `json:",omitempty"`
	Tags  map[string]string `json:",omitempty"`
}

// NodePool is a Karpenter NodePool inventory object (cluster-scoped).
type NodePool struct {
	Kind         string
	Name         string
	UID          string
	APIVersion   string
	Labels       map[string]string
	Annotations  map[string]string
	Weight       *int32
	Limits       map[string]string
	NodeClassRef *KarpenterNodeClassRef
	Requirements []KarpenterRequirement
	Disruption   *KarpenterDisruption
	Conditions   []KarpenterCondition `json:",omitempty"`
}

// NodeClaim is a Karpenter NodeClaim inventory object (cluster-scoped).
type NodeClaim struct {
	Kind         string
	Name         string
	UID          string
	APIVersion   string
	Labels       map[string]string
	Annotations  map[string]string
	NodePool     string `json:",omitempty"`
	NodeClassRef *KarpenterNodeClassRef
	Requirements []KarpenterRequirement
	ProviderID   string               `json:",omitempty"`
	NodeName     string               `json:",omitempty"`
	Conditions   []KarpenterCondition `json:",omitempty"`
}

// EC2NodeClass is an AWS Karpenter EC2NodeClass inventory object (cluster-scoped).
type EC2NodeClass struct {
	Kind                       string
	Name                       string
	UID                        string
	APIVersion                 string
	Labels                     map[string]string
	Annotations                map[string]string
	Role                       string                  `json:",omitempty"`
	InstanceProfile            string                  `json:",omitempty"`
	AMIFamily                  string                  `json:",omitempty"`
	Tags                       map[string]string       `json:",omitempty"`
	SubnetSelectorTerms        []KarpenterSelectorTerm `json:",omitempty"`
	SecurityGroupSelectorTerms []KarpenterSelectorTerm `json:",omitempty"`
	AMISelectorTerms           []KarpenterSelectorTerm `json:",omitempty"`
	Conditions                 []KarpenterCondition    `json:",omitempty"`
}

// Karpenter is optional Karpenter inventory nested under ClusterWorkloadReport.
// Omitted (nil) when karpenter.sh CRDs are not installed.
type Karpenter struct {
	NodePools      []NodePool
	NodeClaims     []NodeClaim
	EC2NodeClasses []EC2NodeClass // AWS only; empty when EC2NodeClass CRDs/RBAC absent
}

// listKarpenterInventory returns nil when karpenter.sh is not installed (NodePool and
// NodeClaim lists both report absent CRDs). When Karpenter is present, nested arrays are
// always set (possibly empty). Forbidden list is treated as present-but-unreadable.
func listKarpenterInventory(ctx context.Context, dynamicClient dynamic.Interface) *Karpenter {
	nodePools, poolsErr := listKarpenterNodePools(ctx, dynamicClient)
	nodeClaims, claimsErr := listKarpenterNodeClaims(ctx, dynamicClient)

	if poolsErr != nil && claimsErr != nil && isKarpenterAbsent(poolsErr) && isKarpenterAbsent(claimsErr) {
		return nil
	}

	if poolsErr != nil {
		logrus.Warnf("error listing Karpenter NodePools, continuing with empty NodePools: %v", poolsErr)
		nodePools = []NodePool{}
	}
	if claimsErr != nil {
		logrus.Warnf("error listing Karpenter NodeClaims, continuing with empty NodeClaims: %v", claimsErr)
		nodeClaims = []NodeClaim{}
	}

	ec2NodeClasses, ec2Err := listEC2NodeClasses(ctx, dynamicClient)
	if ec2Err != nil {
		if !isKarpenterAbsent(ec2Err) {
			logrus.Warnf("error listing EC2NodeClasses, continuing with empty EC2NodeClasses: %v", ec2Err)
		}
		ec2NodeClasses = []EC2NodeClass{}
	}

	return &Karpenter{
		NodePools:      nodePools,
		NodeClaims:     nodeClaims,
		EC2NodeClasses: ec2NodeClasses,
	}
}

func isKarpenterAbsent(err error) bool {
	return apierrors.IsNotFound(err) || meta.IsNoMatchError(err)
}

func listKarpenterNodePools(ctx context.Context, dynamicClient dynamic.Interface) ([]NodePool, error) {
	items, err := listClusterUnstructured(ctx, dynamicClient, nodePoolGVR)
	if err != nil {
		return nil, err
	}
	out := make([]NodePool, 0, len(items))
	for _, item := range items {
		out = append(out, formatNodePool(item))
	}
	return out, nil
}

func listKarpenterNodeClaims(ctx context.Context, dynamicClient dynamic.Interface) ([]NodeClaim, error) {
	items, err := listClusterUnstructured(ctx, dynamicClient, nodeClaimGVR)
	if err != nil {
		return nil, err
	}
	out := make([]NodeClaim, 0, len(items))
	for _, item := range items {
		out = append(out, formatNodeClaim(item))
	}
	return out, nil
}

func listEC2NodeClasses(ctx context.Context, dynamicClient dynamic.Interface) ([]EC2NodeClass, error) {
	items, err := listClusterUnstructured(ctx, dynamicClient, ec2NodeClassGVR)
	if err != nil {
		return nil, err
	}
	out := make([]EC2NodeClass, 0, len(items))
	for _, item := range items {
		out = append(out, formatEC2NodeClass(item))
	}
	return out, nil
}

func listClusterUnstructured(
	ctx context.Context,
	dynamicClient dynamic.Interface,
	gvr schema.GroupVersionResource,
) ([]unstructured.Unstructured, error) {
	var items []unstructured.Unstructured
	continueToken := ""
	for {
		list, err := dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{Continue: continueToken})
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

func formatNodePool(item unstructured.Unstructured) NodePool {
	apiVersion := item.GetAPIVersion()
	if apiVersion == "" {
		apiVersion = karpenterAPIVersion
	}
	np := NodePool{
		Kind:         KindNodePool,
		Name:         item.GetName(),
		UID:          string(item.GetUID()),
		APIVersion:   apiVersion,
		Labels:       item.GetLabels(),
		Annotations:  item.GetAnnotations(),
		Limits:       nestedStringMapTolerant(item.Object, "spec", "limits"),
		NodeClassRef: formatNodeClassRef(nestedMap(item.Object, "spec", "template", "spec", "nodeClassRef")),
		Requirements: formatRequirements(nestedSlice(item.Object, "spec", "template", "spec", "requirements")),
		Disruption:   formatDisruption(nestedMap(item.Object, "spec", "disruption")),
		Conditions:   formatKarpenterConditions(nestedSlice(item.Object, "status", "conditions")),
	}
	if weight, ok, _ := unstructured.NestedInt64(item.Object, "spec", "weight"); ok {
		w := int32(weight)
		np.Weight = &w
	}
	return np
}

func formatNodeClaim(item unstructured.Unstructured) NodeClaim {
	apiVersion := item.GetAPIVersion()
	if apiVersion == "" {
		apiVersion = karpenterAPIVersion
	}
	labels := item.GetLabels()
	nodePool := ""
	if labels != nil {
		nodePool = labels[karpenterNodePoolLabelKey]
	}
	return NodeClaim{
		Kind:         KindNodeClaim,
		Name:         item.GetName(),
		UID:          string(item.GetUID()),
		APIVersion:   apiVersion,
		Labels:       labels,
		Annotations:  item.GetAnnotations(),
		NodePool:     nodePool,
		NodeClassRef: formatNodeClassRef(nestedMap(item.Object, "spec", "nodeClassRef")),
		Requirements: formatRequirements(nestedSlice(item.Object, "spec", "requirements")),
		ProviderID:   nestedString(item.Object, "status", "providerID"),
		NodeName:     nestedString(item.Object, "status", "nodeName"),
		Conditions:   formatKarpenterConditions(nestedSlice(item.Object, "status", "conditions")),
	}
}

func formatEC2NodeClass(item unstructured.Unstructured) EC2NodeClass {
	apiVersion := item.GetAPIVersion()
	if apiVersion == "" {
		apiVersion = ec2NodeClassAPIVersion
	}
	return EC2NodeClass{
		Kind:                       KindEC2NodeClass,
		Name:                       item.GetName(),
		UID:                        string(item.GetUID()),
		APIVersion:                 apiVersion,
		Labels:                     item.GetLabels(),
		Annotations:                item.GetAnnotations(),
		Role:                       nestedString(item.Object, "spec", "role"),
		InstanceProfile:            nestedString(item.Object, "spec", "instanceProfile"),
		AMIFamily:                  nestedString(item.Object, "spec", "amiFamily"),
		Tags:                       nestedStringMapTolerant(item.Object, "spec", "tags"),
		SubnetSelectorTerms:        formatSelectorTerms(nestedSlice(item.Object, "spec", "subnetSelectorTerms")),
		SecurityGroupSelectorTerms: formatSelectorTerms(nestedSlice(item.Object, "spec", "securityGroupSelectorTerms")),
		AMISelectorTerms:           formatSelectorTerms(nestedSlice(item.Object, "spec", "amiSelectorTerms")),
		Conditions:                 formatKarpenterConditions(nestedSlice(item.Object, "status", "conditions")),
	}
}

func formatNodeClassRef(raw map[string]any) *KarpenterNodeClassRef {
	if len(raw) == 0 {
		return nil
	}
	ref := &KarpenterNodeClassRef{
		Group: asString(raw["group"]),
		Kind:  asString(raw["kind"]),
		Name:  asString(raw["name"]),
	}
	if ref.Group == "" && ref.Kind == "" && ref.Name == "" {
		return nil
	}
	return ref
}

func formatRequirements(raw []any) []KarpenterRequirement {
	if len(raw) == 0 {
		return nil
	}
	out := make([]KarpenterRequirement, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		req := KarpenterRequirement{
			Key:      asString(m["key"]),
			Operator: asString(m["operator"]),
			Values:   asStringSlice(m["values"]),
		}
		if req.Key == "" && req.Operator == "" && len(req.Values) == 0 {
			continue
		}
		out = append(out, req)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func formatDisruption(raw map[string]any) *KarpenterDisruption {
	if len(raw) == 0 {
		return nil
	}
	d := &KarpenterDisruption{
		ConsolidationPolicy: asString(raw["consolidationPolicy"]),
		ConsolidateAfter:    asString(raw["consolidateAfter"]),
	}
	if budgets, ok := raw["budgets"].([]any); ok {
		for _, b := range budgets {
			bm, ok := b.(map[string]any)
			if !ok {
				continue
			}
			d.Budgets = append(d.Budgets, KarpenterDisruptionBudget{
				Nodes:    asString(bm["nodes"]),
				Schedule: asString(bm["schedule"]),
				Duration: asString(bm["duration"]),
			})
		}
	}
	if d.ConsolidationPolicy == "" && d.ConsolidateAfter == "" && len(d.Budgets) == 0 {
		return nil
	}
	return d
}

func formatSelectorTerms(raw []any) []KarpenterSelectorTerm {
	if len(raw) == 0 {
		return nil
	}
	out := make([]KarpenterSelectorTerm, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		term := KarpenterSelectorTerm{
			ID:    asString(m["id"]),
			Name:  asString(m["name"]),
			Alias: asString(m["alias"]),
			Owner: asString(m["owner"]),
			Tags:  asStringMap(m["tags"]),
		}
		if term.ID == "" && term.Name == "" && term.Alias == "" && term.Owner == "" && len(term.Tags) == 0 {
			continue
		}
		out = append(out, term)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func formatKarpenterConditions(raw []any) []KarpenterCondition {
	if len(raw) == 0 {
		return nil
	}
	out := make([]KarpenterCondition, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, KarpenterCondition{
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

func nestedMap(obj map[string]any, fields ...string) map[string]any {
	m, ok, err := unstructured.NestedMap(obj, fields...)
	if err != nil || !ok {
		return nil
	}
	return m
}

func nestedSlice(obj map[string]any, fields ...string) []any {
	s, ok, err := unstructured.NestedSlice(obj, fields...)
	if err != nil || !ok {
		return nil
	}
	return s
}

func nestedString(obj map[string]any, fields ...string) string {
	s, ok, err := unstructured.NestedString(obj, fields...)
	if err != nil || !ok {
		return ""
	}
	return s
}

// nestedStringMapTolerant reads a nested map and stringifies values so resource
// quantities encoded as numbers still appear in inventory output.
func nestedStringMapTolerant(obj map[string]any, fields ...string) map[string]string {
	m, ok, err := unstructured.NestedMap(obj, fields...)
	if err != nil || !ok || len(m) == 0 {
		return nil
	}
	return asStringMap(m)
}

func asStringMap(v any) map[string]string {
	raw, ok := v.(map[string]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, val := range raw {
		s := asString(val)
		if s == "" {
			continue
		}
		out[k] = s
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func asStringSlice(v any) []string {
	raw, ok := v.([]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		s := asString(item)
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

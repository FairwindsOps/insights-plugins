package workloads

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/fairwindsops/controller-utils/pkg/controller"
	"github.com/fairwindsops/insights-plugins/plugins/workloads/pkg/discovery"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const (
	KindIngress                 = "Ingress"
	networkingIngressAPIVersion = "networking.k8s.io/v1"
)

// ControllerResult provides a wrapper around a PodResult
type ControllerResult struct {
	Kind           string
	Name           string
	Namespace      string
	Annotations    map[string]string
	Labels         map[string]string
	PodLabels      map[string]string
	PodAnnotations map[string]string
	UID            string
	ParentUID      string
	PodCount       float64
	Containers     []ContainerResult
}

// IngressBackendSummary is a service or resource backend for an Ingress path/default.
type IngressBackendSummary struct {
	ServiceName string `json:",omitempty"`
	ServicePort string `json:",omitempty"`
	API         string `json:",omitempty"`
	Kind        string `json:",omitempty"`
	Name        string `json:",omitempty"`
}

// IngressPathSummary is a single HTTP path rule.
type IngressPathSummary struct {
	Path     string
	PathType string `json:",omitempty"`
	Backend  IngressBackendSummary
}

// IngressRuleSummary is a host rule with HTTP paths.
type IngressRuleSummary struct {
	Host  string
	Paths []IngressPathSummary `json:",omitempty"`
}

// IngressTLSSummary is TLS config for an Ingress.
type IngressTLSSummary struct {
	Hosts      []string `json:",omitempty"`
	SecretName string   `json:",omitempty"`
}

// IngressLoadBalancerEntry is a load-balancer ingress point.
type IngressLoadBalancerEntry struct {
	IP       string `json:",omitempty"`
	Hostname string `json:",omitempty"`
}

type Ingress struct {
	Kind             string
	Name             string
	Namespace        string
	Annotations      map[string]string
	Labels           map[string]string
	UID              string
	APIVersion       string
	IngressClassName *string                   `json:",omitempty"`
	Rules            []IngressRuleSummary      `json:",omitempty"`
	TLS              []IngressTLSSummary       `json:",omitempty"`
	DefaultBackend   *IngressBackendSummary    `json:",omitempty"`
	LoadBalancer     []IngressLoadBalancerEntry `json:",omitempty"`
}

// ContainerResult provides a list of validation messages for each container.
type ContainerResult struct {
	Name         string
	Image        string
	ImageID      string
	CreationTime time.Time
	Resource     ResourceResult
}

// AppliedResources are request/limit strings derived from
// status.containerStatuses[].resources (runtime-applied), not pod spec.
// CPU and Memory stay on Requests/Limits (ResourcesInfo). GPU-class applied quantities use ExtendedRequests/ExtendedLimits.
type AppliedResources struct {
	Requests         ResourcesInfo
	Limits           ResourcesInfo
	ExtendedRequests map[string]string `json:"ExtendedRequests,omitempty"`
	ExtendedLimits   map[string]string `json:"ExtendedLimits,omitempty"`
}

// ResourceResult provides resources information.
type ResourceResult struct {
	Requests ResourcesInfo
	Limits   ResourcesInfo
	// GPURequests / GPULimits: optional template (pod spec) GPU-class maps; resource names vary by vendor (see Extended* on Applied for applied status when skewing).
	GPURequests map[string]string `json:"GPURequests,omitempty"`
	GPULimits   map[string]string `json:"GPULimits,omitempty"`
	// SpecAppliedConvergedCount is how many Running+Ready pods have status.containerStatuses[].resources
	// populated and semantically matching this workload's pod template (CPU/memory and tracked GPU-class resources) for this container.
	SpecAppliedConvergedCount int `json:"SpecAppliedConvergedCount"`
	// SpecAppliedSkewPods lists pods whose applied status resources differ from the top controller pod template (e.g. in-place resize ahead of template rollout).
	SpecAppliedSkewPods []SpecAppliedSkewPod `json:",omitempty"`
}

// ResourcesInfo provides request/limit item information (CPU and memory only).
type ResourcesInfo struct {
	Memory string
	CPU    string
}

// NodeConditionSummary is a simplified node condition.
type NodeConditionSummary struct {
	Type               string
	Status             string
	Reason             string    `json:",omitempty"`
	Message            string    `json:",omitempty"`
	LastTransitionTime time.Time `json:",omitempty"`
}

// NodeTaintSummary is a simplified node taint.
type NodeTaintSummary struct {
	Key    string
	Value  string `json:",omitempty"`
	Effect string
}

// NodeAddressSummary is a simplified node address.
type NodeAddressSummary struct {
	Type    string
	Address string
}

// NodeInfoSummary is a simplified node system info.
type NodeInfoSummary struct {
	Architecture            string `json:",omitempty"`
	OperatingSystem         string `json:",omitempty"`
	OSImage                 string `json:",omitempty"`
	ContainerRuntimeVersion string `json:",omitempty"`
	KernelVersion           string `json:",omitempty"`
	KubeletVersion          string `json:",omitempty"`
	KubeProxyVersion        string `json:",omitempty"`
}

// NodeSummary gives highlevel overview of node informations
type NodeSummary struct {
	Name               string
	UID                string
	Labels             map[string]string
	Annotations        map[string]string
	CreationTimestamp  time.Time
	Capacity           corev1.ResourceList
	Allocatable        corev1.ResourceList
	AllocatedLimits    corev1.ResourceList
	AllocatedRequests  corev1.ResourceList
	Utilization        NodeUtilization
	KubeletVersion     string
	KubeProxyVersion   string
	IsControlPlaneNode bool
	Conditions         []NodeConditionSummary `json:",omitempty"`
	Taints             []NodeTaintSummary     `json:",omitempty"`
	Unschedulable      bool
	Addresses          []NodeAddressSummary   `json:",omitempty"`
	ProviderID         string                 `json:",omitempty"`
	NodeInfo           NodeInfoSummary
}

// NamespaceCounts holds per-namespace inventory object counts.
type NamespaceCounts struct {
	Name               string
	ResourceQuotaCount int
	LimitRangeCount    int
	NetworkPolicyCount int
	PodCount           int
	ServiceCount       int
	IngressCount       int
}

// ClusterWorkloadReport contains k8s workload resources report structure
type ClusterWorkloadReport struct {
	ServerVersion   string
	CreationTime    time.Time
	SourceName      string
	SourceType      string
	Nodes           []NodeSummary
	Namespaces      []corev1.Namespace
	NamespaceCounts []NamespaceCounts
	Controllers     []ControllerResult
	Ingresses       []Ingress
	Images          []discovery.ImageResult
}

func getOwnerUID(ownerReferences []metav1.OwnerReference) string {
	ownerUID := ""
	if len(ownerReferences) > 0 {
		ownerUID = string(ownerReferences[0].UID)
	}
	return ownerUID
}

func formatControllers(kind, name, namespace, uid string, ownerReferences []metav1.OwnerReference, containers []ContainerResult, annotations, labels, podLabels, podAnnotations map[string]string) ControllerResult {
	var podCount float64 = 0
	if kind == "Pod" {
		podCount = 1
	}
	ownerUID := getOwnerUID(ownerReferences)
	return ControllerResult{kind, name, namespace, annotations, labels, podLabels, podAnnotations, uid, ownerUID, podCount, containers}
}

func resourcesFromContainerSpec(container corev1.Container) ResourceResult {
	resources := ResourceResult{
		Requests: ResourcesInfo{
			CPU:    container.Resources.Requests.Cpu().String(),
			Memory: container.Resources.Requests.Memory().String(),
		},
		Limits: ResourcesInfo{
			CPU:    container.Resources.Limits.Cpu().String(),
			Memory: container.Resources.Limits.Memory().String(),
		},
	}
	if container.Resources.Requests.Cpu().IsZero() && !container.Resources.Limits.Cpu().IsZero() {
		resources.Requests.CPU = resources.Limits.CPU
	}
	if container.Resources.Requests.Memory().IsZero() && !container.Resources.Limits.Memory().IsZero() {
		resources.Requests.Memory = resources.Limits.Memory
	}
	gpuReq, gpuLim := extendedGPUMapsFromResourceRequirements(&container.Resources)
	resources.GPURequests = gpuReq
	resources.GPULimits = gpuLim
	return resources
}

func formatContainer(container corev1.Container, containerStatus corev1.ContainerStatus, time metav1.Time, specApplied SpecAppliedStats) ContainerResult {
	resources := resourcesFromContainerSpec(container)
	resources.SpecAppliedConvergedCount = specApplied.ConvergedCount
	if len(specApplied.SkewPods) > 0 {
		resources.SpecAppliedSkewPods = specApplied.SkewPods
	}
	containerResult := ContainerResult{
		Name:         container.Name,
		Image:        container.Image,
		ImageID:      containerStatus.ImageID,
		CreationTime: time.UTC(),
		Resource:     resources,
	}

	return containerResult
}

func formatNodeConditions(conditions []corev1.NodeCondition) []NodeConditionSummary {
	if len(conditions) == 0 {
		return nil
	}
	out := make([]NodeConditionSummary, 0, len(conditions))
	for _, c := range conditions {
		out = append(out, NodeConditionSummary{
			Type:               string(c.Type),
			Status:             string(c.Status),
			Reason:             c.Reason,
			Message:            c.Message,
			LastTransitionTime: c.LastTransitionTime.UTC(),
		})
	}
	return out
}

func formatNodeTaints(taints []corev1.Taint) []NodeTaintSummary {
	if len(taints) == 0 {
		return nil
	}
	out := make([]NodeTaintSummary, 0, len(taints))
	for _, t := range taints {
		out = append(out, NodeTaintSummary{
			Key:    t.Key,
			Value:  t.Value,
			Effect: string(t.Effect),
		})
	}
	return out
}

func formatNodeAddresses(addresses []corev1.NodeAddress) []NodeAddressSummary {
	if len(addresses) == 0 {
		return nil
	}
	out := make([]NodeAddressSummary, 0, len(addresses))
	for _, a := range addresses {
		out = append(out, NodeAddressSummary{
			Type:    string(a.Type),
			Address: a.Address,
		})
	}
	return out
}

func formatNodeInfo(info corev1.NodeSystemInfo) NodeInfoSummary {
	return NodeInfoSummary{
		Architecture:            info.Architecture,
		OperatingSystem:         info.OperatingSystem,
		OSImage:                 info.OSImage,
		ContainerRuntimeVersion: info.ContainerRuntimeVersion,
		KernelVersion:           info.KernelVersion,
		KubeletVersion:          info.KubeletVersion,
		KubeProxyVersion:        info.KubeProxyVersion,
	}
}

// serviceBackendPortString converts a networking/v1 ServiceBackendPort to a string
// (port name if set, otherwise the numeric port).
func serviceBackendPortString(port networkingv1.ServiceBackendPort) string {
	if port.Name != "" {
		return port.Name
	}
	if port.Number != 0 {
		return strconv.FormatInt(int64(port.Number), 10)
	}
	return ""
}

func formatIngressBackend(backend *networkingv1.IngressBackend) *IngressBackendSummary {
	if backend == nil {
		return nil
	}
	out := IngressBackendSummary{}
	if backend.Service != nil {
		out.ServiceName = backend.Service.Name
		out.ServicePort = serviceBackendPortString(backend.Service.Port)
	}
	if backend.Resource != nil {
		if backend.Resource.APIGroup != nil {
			out.API = *backend.Resource.APIGroup
		}
		out.Kind = backend.Resource.Kind
		out.Name = backend.Resource.Name
	}
	return &out
}

func formatIngressRules(rules []networkingv1.IngressRule) []IngressRuleSummary {
	if len(rules) == 0 {
		return nil
	}
	out := make([]IngressRuleSummary, 0, len(rules))
	for _, rule := range rules {
		summary := IngressRuleSummary{Host: rule.Host}
		if rule.HTTP != nil {
			for _, p := range rule.HTTP.Paths {
				pathType := ""
				if p.PathType != nil {
					pathType = string(*p.PathType)
				}
				backend := formatIngressBackend(&p.Backend)
				pathSummary := IngressPathSummary{
					Path:     p.Path,
					PathType: pathType,
				}
				if backend != nil {
					pathSummary.Backend = *backend
				}
				summary.Paths = append(summary.Paths, pathSummary)
			}
		}
		out = append(out, summary)
	}
	return out
}

func formatIngressTLS(tls []networkingv1.IngressTLS) []IngressTLSSummary {
	if len(tls) == 0 {
		return nil
	}
	out := make([]IngressTLSSummary, 0, len(tls))
	for _, t := range tls {
		out = append(out, IngressTLSSummary{
			Hosts:      t.Hosts,
			SecretName: t.SecretName,
		})
	}
	return out
}

func formatIngressLoadBalancer(status networkingv1.IngressLoadBalancerStatus) []IngressLoadBalancerEntry {
	if len(status.Ingress) == 0 {
		return nil
	}
	out := make([]IngressLoadBalancerEntry, 0, len(status.Ingress))
	for _, lb := range status.Ingress {
		out = append(out, IngressLoadBalancerEntry{
			IP:       lb.IP,
			Hostname: lb.Hostname,
		})
	}
	return out
}

func resolveIngressAPIVersion(item networkingv1.Ingress) string {
	if item.APIVersion != "" {
		return item.APIVersion
	}
	if len(item.ManagedFields) > 0 && item.ManagedFields[0].APIVersion != "" {
		return item.ManagedFields[0].APIVersion
	}
	return networkingIngressAPIVersion
}

func formatIngress(item networkingv1.Ingress) Ingress {
	return Ingress{
		Kind:             KindIngress,
		Name:             item.Name,
		Namespace:        item.Namespace,
		Annotations:      item.Annotations,
		Labels:           item.Labels,
		UID:              string(item.UID),
		APIVersion:       resolveIngressAPIVersion(item),
		IngressClassName: item.Spec.IngressClassName,
		Rules:            formatIngressRules(item.Spec.Rules),
		TLS:              formatIngressTLS(item.Spec.TLS),
		DefaultBackend:   formatIngressBackend(item.Spec.DefaultBackend),
		LoadBalancer:     formatIngressLoadBalancer(item.Status.LoadBalancer),
	}
}

type namespaceCountAccum struct {
	ResourceQuotaCount int
	LimitRangeCount    int
	NetworkPolicyCount int
	PodCount           int
	ServiceCount       int
	IngressCount       int
}

func collectNamespaceCounts(ctx context.Context, kube kubernetes.Interface, namespaces []corev1.Namespace, ingresses []Ingress) ([]NamespaceCounts, error) {
	listOpts := metav1.ListOptions{}
	countsByNS := make(map[string]*namespaceCountAccum, len(namespaces))
	for _, ns := range namespaces {
		countsByNS[ns.Name] = &namespaceCountAccum{}
	}

	pods, err := kube.CoreV1().Pods(metav1.NamespaceAll).List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("error listing pods for namespace counts: %w", err)
	}
	for _, pod := range pods.Items {
		if c, ok := countsByNS[pod.Namespace]; ok {
			c.PodCount++
		}
	}

	services, err := kube.CoreV1().Services(metav1.NamespaceAll).List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("error listing services for namespace counts: %w", err)
	}
	for _, svc := range services.Items {
		if c, ok := countsByNS[svc.Namespace]; ok {
			c.ServiceCount++
		}
	}

	resourceQuotas, err := kube.CoreV1().ResourceQuotas(metav1.NamespaceAll).List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("error listing resource quotas for namespace counts: %w", err)
	}
	for _, rq := range resourceQuotas.Items {
		if c, ok := countsByNS[rq.Namespace]; ok {
			c.ResourceQuotaCount++
		}
	}

	limitRanges, err := kube.CoreV1().LimitRanges(metav1.NamespaceAll).List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("error listing limit ranges for namespace counts: %w", err)
	}
	for _, lr := range limitRanges.Items {
		if c, ok := countsByNS[lr.Namespace]; ok {
			c.LimitRangeCount++
		}
	}

	networkPolicies, err := kube.NetworkingV1().NetworkPolicies(metav1.NamespaceAll).List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("error listing network policies for namespace counts: %w", err)
	}
	for _, np := range networkPolicies.Items {
		if c, ok := countsByNS[np.Namespace]; ok {
			c.NetworkPolicyCount++
		}
	}

	for _, ing := range ingresses {
		if c, ok := countsByNS[ing.Namespace]; ok {
			c.IngressCount++
		}
	}

	out := make([]NamespaceCounts, 0, len(namespaces))
	for _, ns := range namespaces {
		c := countsByNS[ns.Name]
		out = append(out, NamespaceCounts{
			Name:               ns.Name,
			ResourceQuotaCount: c.ResourceQuotaCount,
			LimitRangeCount:    c.LimitRangeCount,
			NetworkPolicyCount: c.NetworkPolicyCount,
			PodCount:           c.PodCount,
			ServiceCount:       c.ServiceCount,
			IngressCount:       c.IngressCount,
		})
	}
	return out, nil
}

// CreateResourceProviderFromAPI creates a new ResourceProvider from an existing k8s interface
func CreateResourceProviderFromAPI(ctx context.Context, dynamicClient dynamic.Interface, restMapper meta.RESTMapper, kube kubernetes.Interface, clusterName string) (*ClusterWorkloadReport, error) {
	listOpts := metav1.ListOptions{}
	interfaces := []ControllerResult{}
	serverVersion, err := kube.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("error fetching Cluster API version: %v", err)
	}

	client := controller.Client{
		Context:    ctx,
		Dynamic:    dynamicClient,
		RESTMapper: restMapper,
	}
	workloads, err := client.GetAllTopControllersWithPods("")
	if err != nil {
		return nil, fmt.Errorf("error while getting all TopControllers: %v", err)
	}

	for _, workload := range workloads {
		topController := workload.TopController

		var containers []ContainerResult
		if workload.PodSpec != nil {
			for _, ctn := range workload.PodSpec.Containers {
				stats := computeSpecAppliedStats(ctn.Name, &ctn.Resources, workload.Pods)
				containers = append(containers, formatContainer(ctn, corev1.ContainerStatus{}, topController.GetCreationTimestamp(), stats))
			}
		}

		var podLabels, podAnnotations map[string]string
		if workload.PodMetadata != nil {
			podLabels = workload.PodMetadata.Labels
			podAnnotations = workload.PodMetadata.Annotations
		}
		controller := formatControllers(topController.GetKind(), topController.GetName(), topController.GetNamespace(), string(topController.GetUID()), topController.GetOwnerReferences(), containers, topController.GetAnnotations(), topController.GetLabels(), podLabels, podAnnotations)
		controller.PodCount = float64(workload.RunningPodCount)
		interfaces = append(interfaces, controller)
	}

	// Nodes
	nodes, err := kube.CoreV1().Nodes().List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("error fetching Nodes: %v", err)
	}

	nodesSummaries := make([]NodeSummary, 0)

	for _, item := range nodes.Items {
		node := NodeSummary{
			Name:               item.GetName(),
			UID:                string(item.UID),
			Labels:             item.GetLabels(),
			Annotations:        item.GetAnnotations(),
			CreationTimestamp:  item.GetCreationTimestamp().UTC(),
			Capacity:           item.Status.Capacity,
			Allocatable:        item.Status.Allocatable,
			KubeletVersion:     item.Status.NodeInfo.KubeletVersion,
			KubeProxyVersion:   item.Status.NodeInfo.KubeProxyVersion,
			IsControlPlaneNode: checkIfNodeIsControlPlane(item.GetLabels()),
			Conditions:         formatNodeConditions(item.Status.Conditions),
			Taints:             formatNodeTaints(item.Spec.Taints),
			Unschedulable:      item.Spec.Unschedulable,
			Addresses:          formatNodeAddresses(item.Status.Addresses),
			ProviderID:         item.Spec.ProviderID,
			NodeInfo:           formatNodeInfo(item.Status.NodeInfo),
		}
		allocated, utilization, err := GetNodeAllocatedResource(ctx, kube, item)
		if err != nil {
			return nil, fmt.Errorf("error fetching node allocation: %v", err)
		}
		node.AllocatedLimits = allocated.Limits
		node.AllocatedRequests = allocated.Requests
		node.Utilization = utilization
		nodesSummaries = append(nodesSummaries, node)
	}

	// Namespaces
	namespaces, err := kube.CoreV1().Namespaces().List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("error fetching Namespaces: %v", err)
	}

	// Ingresses
	ingresses := []Ingress{}
	for _, namespace := range namespaces.Items {
		ingressesV1 := kube.NetworkingV1().Ingresses(namespace.Name)
		list, err := ingressesV1.List(ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("error fetching ingresses: %v", err)
		}
		for _, item := range list.Items {
			ingresses = append(ingresses, formatIngress(item))
		}
	}

	namespaceCounts, err := collectNamespaceCounts(ctx, kube, namespaces.Items, ingresses)
	if err != nil {
		return nil, err
	}

	images := []discovery.ImageResult{}
	imageDiscovery, err := discovery.ListImages(ctx, kube, workloads)
	if err != nil {
		logrus.Warnf("error listing running images, continuing with empty Images: %v", err)
	} else {
		images = imageDiscovery.Images
	}

	clusterWorkloadReport := ClusterWorkloadReport{
		ServerVersion:   serverVersion.Major + "." + serverVersion.Minor,
		SourceType:      "Cluster",
		SourceName:      clusterName,
		CreationTime:    time.Now(),
		Nodes:           nodesSummaries,
		Namespaces:      namespaces.Items,
		NamespaceCounts: namespaceCounts,
		Controllers:     interfaces,
		Ingresses:       ingresses,
		Images:          images,
	}
	return &clusterWorkloadReport, nil
}

func checkIfNodeIsControlPlane(labels map[string]string) bool {
	keys := lo.Keys[string, string](labels)
	return lo.Contains(keys, "node-role.kubernetes.io/control-plane") || lo.Contains(keys, "node-role.kubernetes.io/master")
}

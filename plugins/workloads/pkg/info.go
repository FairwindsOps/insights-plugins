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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const (
	KindIngress                 = "Ingress"
	KindService                 = "Service"
	KindPersistentVolumeClaim   = "PersistentVolumeClaim"
	networkingIngressAPIVersion = "networking.k8s.io/v1"
	coreAPIVersion              = "v1"
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
	IngressClassName *string                    `json:",omitempty"`
	Rules            []IngressRuleSummary       `json:",omitempty"`
	TLS              []IngressTLSSummary        `json:",omitempty"`
	DefaultBackend   *IngressBackendSummary     `json:",omitempty"`
	LoadBalancer     []IngressLoadBalancerEntry `json:",omitempty"`
}

// ServicePortSummary is a single Service port.
type ServicePortSummary struct {
	Name       string `json:",omitempty"`
	Protocol   string `json:",omitempty"`
	Port       int32
	TargetPort string `json:",omitempty"`
	NodePort   int32  `json:",omitempty"`
}

// Service is a cluster Service inventory object.
type Service struct {
	Kind         string
	Name         string
	Namespace    string
	Annotations  map[string]string
	Labels       map[string]string
	UID          string
	APIVersion   string
	Type         string                     `json:",omitempty"`
	ClusterIP    string                     `json:",omitempty"`
	ClusterIPs   []string                   `json:",omitempty"`
	ExternalName string                     `json:",omitempty"`
	ExternalIPs  []string                   `json:",omitempty"`
	Selector     map[string]string          `json:",omitempty"`
	Ports        []ServicePortSummary       `json:",omitempty"`
	LoadBalancer []IngressLoadBalancerEntry `json:",omitempty"`
}

// PersistentVolumeClaim is a PVC inventory object.
type PersistentVolumeClaim struct {
	Kind             string
	Name             string
	Namespace        string
	Annotations      map[string]string
	Labels           map[string]string
	UID              string
	APIVersion       string
	StorageClassName *string  `json:",omitempty"`
	AccessModes      []string `json:",omitempty"`
	VolumeMode       string   `json:",omitempty"`
	VolumeName       string   `json:",omitempty"`
	RequestStorage   string   `json:",omitempty"`
	CapacityStorage  string   `json:",omitempty"`
	Phase            string   `json:",omitempty"`
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
	Addresses          []NodeAddressSummary `json:",omitempty"`
	ProviderID         string               `json:",omitempty"`
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
	ServerVersion          string
	CreationTime           time.Time
	SourceName             string
	SourceType             string
	Nodes                  []NodeSummary
	Namespaces             []corev1.Namespace
	NamespaceCounts        []NamespaceCounts
	Controllers            []ControllerResult
	Ingresses              []Ingress
	Services               []Service
	PersistentVolumeClaims []PersistentVolumeClaim
	Images                 []discovery.ImageResult
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

func resolveCoreAPIVersion(apiVersion string) string {
	if apiVersion != "" {
		return apiVersion
	}
	return coreAPIVersion
}

func intOrStringString(value intstr.IntOrString) string {
	if value.Type == intstr.String {
		return value.StrVal
	}
	if value.IntVal == 0 {
		return ""
	}
	return strconv.FormatInt(int64(value.IntVal), 10)
}

func formatServicePorts(ports []corev1.ServicePort) []ServicePortSummary {
	if len(ports) == 0 {
		return nil
	}
	out := make([]ServicePortSummary, 0, len(ports))
	for _, p := range ports {
		out = append(out, ServicePortSummary{
			Name:       p.Name,
			Protocol:   string(p.Protocol),
			Port:       p.Port,
			TargetPort: intOrStringString(p.TargetPort),
			NodePort:   p.NodePort,
		})
	}
	return out
}

func formatServiceLoadBalancer(status corev1.LoadBalancerStatus) []IngressLoadBalancerEntry {
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

func formatService(item corev1.Service) Service {
	return Service{
		Kind:         KindService,
		Name:         item.Name,
		Namespace:    item.Namespace,
		Annotations:  item.Annotations,
		Labels:       item.Labels,
		UID:          string(item.UID),
		APIVersion:   resolveCoreAPIVersion(item.APIVersion),
		Type:         string(item.Spec.Type),
		ClusterIP:    item.Spec.ClusterIP,
		ClusterIPs:   item.Spec.ClusterIPs,
		ExternalName: item.Spec.ExternalName,
		ExternalIPs:  item.Spec.ExternalIPs,
		Selector:     item.Spec.Selector,
		Ports:        formatServicePorts(item.Spec.Ports),
		LoadBalancer: formatServiceLoadBalancer(item.Status.LoadBalancer),
	}
}

func formatAccessModes(modes []corev1.PersistentVolumeAccessMode) []string {
	if len(modes) == 0 {
		return nil
	}
	out := make([]string, 0, len(modes))
	for _, mode := range modes {
		out = append(out, string(mode))
	}
	return out
}

func formatPersistentVolumeClaim(item corev1.PersistentVolumeClaim) PersistentVolumeClaim {
	volumeMode := ""
	if item.Spec.VolumeMode != nil {
		volumeMode = string(*item.Spec.VolumeMode)
	}
	requestStorage := ""
	if item.Spec.Resources.Requests != nil {
		if qty, ok := item.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			requestStorage = qty.String()
		}
	}
	capacityStorage := ""
	if item.Status.Capacity != nil {
		if qty, ok := item.Status.Capacity[corev1.ResourceStorage]; ok {
			capacityStorage = qty.String()
		}
	}
	return PersistentVolumeClaim{
		Kind:             KindPersistentVolumeClaim,
		Name:             item.Name,
		Namespace:        item.Namespace,
		Annotations:      item.Annotations,
		Labels:           item.Labels,
		UID:              string(item.UID),
		APIVersion:       resolveCoreAPIVersion(item.APIVersion),
		StorageClassName: item.Spec.StorageClassName,
		AccessModes:      formatAccessModes(item.Spec.AccessModes),
		VolumeMode:       volumeMode,
		VolumeName:       item.Spec.VolumeName,
		RequestStorage:   requestStorage,
		CapacityStorage:  capacityStorage,
		Phase:            string(item.Status.Phase),
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

// collectNamespaceCounts builds per-namespace object counts. Pod, Service, and Ingress
// counts are derived from already-fetched lists. ResourceQuotas / LimitRanges /
// NetworkPolicies are listed cluster-wide; list failures are logged and that counter
// is left at 0 so missing RBAC does not fail the whole report.
func collectNamespaceCounts(ctx context.Context, kube kubernetes.Interface, namespaces []corev1.Namespace, pods []corev1.Pod, services []Service, ingresses []Ingress) []NamespaceCounts {
	listOpts := metav1.ListOptions{}
	countsByNS := make(map[string]*namespaceCountAccum, len(namespaces))
	for _, ns := range namespaces {
		countsByNS[ns.Name] = &namespaceCountAccum{}
	}

	for _, pod := range pods {
		if c, ok := countsByNS[pod.Namespace]; ok {
			c.PodCount++
		}
	}

	for _, svc := range services {
		if c, ok := countsByNS[svc.Namespace]; ok {
			c.ServiceCount++
		}
	}

	for _, ing := range ingresses {
		if c, ok := countsByNS[ing.Namespace]; ok {
			c.IngressCount++
		}
	}

	resourceQuotas, err := kube.CoreV1().ResourceQuotas(metav1.NamespaceAll).List(ctx, listOpts)
	if err != nil {
		logrus.Warnf("error listing resource quotas for namespace counts, leaving ResourceQuotaCount at 0: %v", err)
	} else {
		for _, rq := range resourceQuotas.Items {
			if c, ok := countsByNS[rq.Namespace]; ok {
				c.ResourceQuotaCount++
			}
		}
	}

	limitRanges, err := kube.CoreV1().LimitRanges(metav1.NamespaceAll).List(ctx, listOpts)
	if err != nil {
		logrus.Warnf("error listing limit ranges for namespace counts, leaving LimitRangeCount at 0: %v", err)
	} else {
		for _, lr := range limitRanges.Items {
			if c, ok := countsByNS[lr.Namespace]; ok {
				c.LimitRangeCount++
			}
		}
	}

	networkPolicies, err := kube.NetworkingV1().NetworkPolicies(metav1.NamespaceAll).List(ctx, listOpts)
	if err != nil {
		logrus.Warnf("error listing network policies for namespace counts, leaving NetworkPolicyCount at 0: %v", err)
	} else {
		for _, np := range networkPolicies.Items {
			if c, ok := countsByNS[np.Namespace]; ok {
				c.NetworkPolicyCount++
			}
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
	return out
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

	// One cluster-wide pod list powers node allocation and NamespaceCounts.PodCount
	// (avoids an extra full list plus per-node pod lists).
	allPods, err := kube.CoreV1().Pods(metav1.NamespaceAll).List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("error fetching Pods: %v", err)
	}

	nodesSummaries := make([]NodeSummary, 0)

	for _, item := range nodes.Items {
		node := NodeSummary{
			Name:              item.GetName(),
			UID:               string(item.UID),
			Labels:            item.GetLabels(),
			Annotations:       item.GetAnnotations(),
			CreationTimestamp: item.GetCreationTimestamp().UTC(),
			Capacity:          item.Status.Capacity,
			Allocatable:       item.Status.Allocatable,
			KubeletVersion:    item.Status.NodeInfo.KubeletVersion,
			//lint:ignore SA1019 keep top-level field for Insights compatibility (often empty on modern clusters)
			KubeProxyVersion:   item.Status.NodeInfo.KubeProxyVersion,
			IsControlPlaneNode: checkIfNodeIsControlPlane(item.GetLabels()),
			Conditions:         formatNodeConditions(item.Status.Conditions),
			Taints:             formatNodeTaints(item.Spec.Taints),
			Unschedulable:      item.Spec.Unschedulable,
			Addresses:          formatNodeAddresses(item.Status.Addresses),
			ProviderID:         item.Spec.ProviderID,
			NodeInfo:           formatNodeInfo(item.Status.NodeInfo),
		}
		allocated, utilization, err := getNodeAllocatedResources(item, podsScheduledOnNode(allPods.Items, item.Name))
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

	// Ingresses (single cluster-scoped list)
	ingresses := []Ingress{}
	ingressList, err := kube.NetworkingV1().Ingresses(metav1.NamespaceAll).List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("error fetching ingresses: %v", err)
	}
	for _, item := range ingressList.Items {
		ingresses = append(ingresses, formatIngress(item))
	}

	// Services (single cluster-scoped list; also feeds NamespaceCounts.ServiceCount)
	services := []Service{}
	serviceList, err := kube.CoreV1().Services(metav1.NamespaceAll).List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("error fetching services: %v", err)
	}
	for _, item := range serviceList.Items {
		services = append(services, formatService(item))
	}

	// PersistentVolumeClaims (single cluster-scoped list)
	pvcs := []PersistentVolumeClaim{}
	pvcList, err := kube.CoreV1().PersistentVolumeClaims(metav1.NamespaceAll).List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("error fetching persistentvolumeclaims: %v", err)
	}
	for _, item := range pvcList.Items {
		pvcs = append(pvcs, formatPersistentVolumeClaim(item))
	}

	namespaceCounts := collectNamespaceCounts(ctx, kube, namespaces.Items, allPods.Items, services, ingresses)

	images := []discovery.ImageResult{}
	imageDiscovery, err := discovery.ListImages(ctx, kube, workloads)
	if err != nil {
		logrus.Warnf("error listing running images, continuing with empty Images: %v", err)
	} else {
		images = imageDiscovery.Images
	}

	clusterWorkloadReport := ClusterWorkloadReport{
		ServerVersion:          serverVersion.Major + "." + serverVersion.Minor,
		SourceType:             "Cluster",
		SourceName:             clusterName,
		CreationTime:           time.Now(),
		Nodes:                  nodesSummaries,
		Namespaces:             namespaces.Items,
		NamespaceCounts:        namespaceCounts,
		Controllers:            interfaces,
		Ingresses:              ingresses,
		Services:               services,
		PersistentVolumeClaims: pvcs,
		Images:                 images,
	}
	return &clusterWorkloadReport, nil
}

func checkIfNodeIsControlPlane(labels map[string]string) bool {
	keys := lo.Keys[string, string](labels)
	return lo.Contains(keys, "node-role.kubernetes.io/control-plane") || lo.Contains(keys, "node-role.kubernetes.io/master")
}

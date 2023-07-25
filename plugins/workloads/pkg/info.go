package workloads

import (
	"context"
	"fmt"
	"time"

	"github.com/fairwindsops/controller-utils/pkg/controller"
	"github.com/thoas/go-funk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const KindIngress = "Ingress"

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

type Ingress struct {
	Kind        string
	Name        string
	Namespace   string
	Annotations map[string]string
	Labels      map[string]string
	UID         string
	APIVersion  string
}

// ContainerResult provides a list of validation messages for each container.
type ContainerResult struct {
	Name         string
	Image        string
	ImageID      string
	CreationTime time.Time
	Resource     ResourceResult
}

// ResourceResult provides resources information.
type ResourceResult struct {
	Requests ResourcesInfo
	Limits   ResourcesInfo
}

// ResourcesInfo provides a request/limit item information.
type ResourcesInfo struct {
	Memory string
	CPU    string
}

// NodeSummary gives highlevel overview of node informations
type NodeSummary struct {
	Name               string
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
}

// ClusterWorkloadReport contains k8s workload resources report structure
type ClusterWorkloadReport struct {
	ServerVersion string
	CreationTime  time.Time
	SourceName    string
	SourceType    string
	Nodes         []NodeSummary
	Namespaces    []corev1.Namespace
	Controllers   []ControllerResult
	Ingresses     []Ingress
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

func formatContainer(container corev1.Container, containerStatus corev1.ContainerStatus, time metav1.Time) ContainerResult {
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
	containerResult := ContainerResult{
		Name:         container.Name,
		Image:        container.Image,
		ImageID:      containerStatus.ImageID,
		CreationTime: time.UTC(),
		Resource:     resources,
	}

	return containerResult
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
	workloads, err := client.GetAllTopControllersSummary("")
	if err != nil {
		return nil, fmt.Errorf("error while getting all TopControllers: %v", err)
	}

	for _, workload := range workloads {
		topController := workload.TopController

		var containers []ContainerResult
		if workload.PodSpec != nil {
			for _, ctn := range workload.PodSpec.Containers {
				containers = append(containers, formatContainer(ctn, corev1.ContainerStatus{}, topController.GetCreationTimestamp()))
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
			Labels:             item.GetLabels(),
			Annotations:        item.GetAnnotations(),
			CreationTimestamp:  item.GetCreationTimestamp().UTC(),
			Capacity:           item.Status.Capacity,
			Allocatable:        item.Status.Allocatable,
			KubeletVersion:     item.Status.NodeInfo.KubeletVersion,
			KubeProxyVersion:   item.Status.NodeInfo.KubeProxyVersion,
			IsControlPlaneNode: checkIfNodeIsControlPlane(item.GetLabels()),
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
			ingress := Ingress{
				Kind:        KindIngress,
				Name:        item.Name,
				Namespace:   item.Namespace,
				Annotations: item.Annotations,
				Labels:      item.Labels,
				UID:         string(item.UID),
			}
			if len(item.ManagedFields) > 0 {
				ingress.APIVersion = item.ManagedFields[0].APIVersion
			}
			ingresses = append(ingresses, ingress)
		}
	}

	clusterWorkloadReport := ClusterWorkloadReport{
		ServerVersion: serverVersion.Major + "." + serverVersion.Minor,
		SourceType:    "Cluster",
		SourceName:    clusterName,
		CreationTime:  time.Now(),
		Nodes:         nodesSummaries,
		Namespaces:    namespaces.Items,
		Controllers:   interfaces,
		Ingresses:     ingresses,
	}
	return &clusterWorkloadReport, nil
}

func checkIfNodeIsControlPlane(labels map[string]string) bool {
	return funk.Contains(labels, "node-role.kubernetes.io/control-plane") || funk.Contains(labels, "node-role.kubernetes.io/master")
}

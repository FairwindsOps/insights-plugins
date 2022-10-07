package main

import (
	"context"
	"time"

	"github.com/fairwindsops/controller-utils/pkg/controller"
	"github.com/sirupsen/logrus"
	"github.com/thoas/go-funk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// ControllerResult provides a wrapper around a PodResult
type ControllerResult struct {
	Kind        string
	Name        string
	Namespace   string
	Annotations map[string]string
	Labels      map[string]string
	UID         string
	ParentUID   string
	PodCount    float64
	Containers  []ContainerResult
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
}

func getOwnerUID(ownerReferences []metav1.OwnerReference) string {
	ownerUID := ""
	if len(ownerReferences) > 0 {
		ownerUID = string(ownerReferences[0].UID)
	}
	return ownerUID
}

func formatControllers(Kind string, Name string, Namespace string, UID string, ownerReferences []metav1.OwnerReference,
	Containers []ContainerResult, Annotations map[string]string, Labels map[string]string) ControllerResult {
	var podCount float64 = 0
	if Kind == "Pod" {
		podCount = 1
	}
	ownerUID := getOwnerUID(ownerReferences)
	controller := ControllerResult{Kind, Name, Namespace, Annotations, Labels, UID, ownerUID, podCount, Containers}
	return controller
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
		logrus.Errorf("Error fetching Cluster API version %v", err)
		return nil, err
	}

	workloads, err := controller.GetAllTopControllers(ctx, dynamicClient, restMapper, "")
	if err != nil {
		logrus.Errorf("Error while getting all TopControllers: %v", err)
		return nil, err
	}

	for _, workload := range workloads {
		topController := workload.TopController
		var containers []ContainerResult
		podCount := float64(len(workload.Pods))

		var pd corev1.Pod

		podSpec := controller.GetPodSpec(workload.TopController.Object)
		if podSpec == nil {
			// Could be a top-level object like Prometheus, which doesn't have controllers.
			if len(workload.Pods) > 0 {
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(workload.Pods[0].UnstructuredContent(), &pd)
				if err != nil {
					return nil, err
				}
			} else {
				// Nothing we can do here--no pods in the cluster, and no containers in the top-level object
				// TODO: there's probably a mid-level object where we can get the info.
				// e.g. a Prometheus doesn't have containers, but its Deployment does
				continue
			}
		} else {
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(map[string]interface{}{"spec": podSpec}, &pd)
			if err != nil {
				return nil, err
			}
		}
		// Convert the unstructured object to cluster.
		for _, ctn := range pd.Spec.Containers {
			containers = append(containers, formatContainer(ctn, corev1.ContainerStatus{}, topController.GetCreationTimestamp()))
		}
		controller := formatControllers(topController.GetKind(), topController.GetName(), topController.GetNamespace(), string(topController.GetUID()), topController.GetOwnerReferences(), containers, topController.GetAnnotations(), topController.GetLabels())
		controller.PodCount = podCount
		interfaces = append(interfaces, controller)
	}

	// Nodes
	nodes, err := kube.CoreV1().Nodes().List(ctx, listOpts)
	if err != nil {
		logrus.Errorf("Error fetching Nodes %v", err)
		return nil, err
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
			logrus.Errorf("Error fetching node allocation: %v", err)
			return nil, err
		}
		node.AllocatedLimits = allocated.Limits
		node.AllocatedRequests = allocated.Requests
		node.Utilization = utilization
		nodesSummaries = append(nodesSummaries, node)
	}

	// Namespaces
	namespaces, err := kube.CoreV1().Namespaces().List(ctx, listOpts)
	if err != nil {
		logrus.Errorf("Error fetching Namespaces %v", err)
		return nil, err
	}

	clusterWorkloadReport := ClusterWorkloadReport{
		ServerVersion: serverVersion.Major + "." + serverVersion.Minor,
		SourceType:    "Cluster",
		SourceName:    clusterName,
		CreationTime:  time.Now(),
		Nodes:         nodesSummaries,
		Namespaces:    namespaces.Items,
		Controllers:   interfaces,
	}
	return &clusterWorkloadReport, nil
}

func checkIfNodeIsControlPlane(labels map[string]string) bool {
	return funk.Contains(labels, "node-role.kubernetes.io/control-plane") || funk.Contains(labels, "node-role.kubernetes.io/master")
}

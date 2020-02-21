package main

import (
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Name              string
	Labels            map[string]string
	Annotations       map[string]string
	CreationTimestamp time.Time
	Capacity          corev1.ResourceList
	Allocatable       corev1.ResourceList
	AllocatedLimits   corev1.ResourceList
	AllocatedRequests corev1.ResourceList
	Utilization       NodeUtilization
	KubeletVersion    string
	KubeProxyVersion  string
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

func formatControllers(Kind string, Name string, Namespace string, UID string, ownerReferences []metav1.OwnerReference,
	Containers []ContainerResult, Annotations map[string]string, Labels map[string]string) ControllerResult {
	ownerUID := ""
	if len(ownerReferences) > 0 {
		ownerUID = string(ownerReferences[0].UID)
	}
	controller := ControllerResult{Kind, Name, Namespace, Annotations, Labels, UID, ownerUID, Containers}
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
func CreateResourceProviderFromAPI(kube kubernetes.Interface, clusterName string) (*ClusterWorkloadReport, error) {
	listOpts := metav1.ListOptions{}
	interfaces := []ControllerResult{}
	serverVersion, err := kube.Discovery().ServerVersion()
	if err != nil {
		logrus.Errorf("Error fetching Cluster API version %v", err)
		return nil, err
	}

	// Deployments
	deploys, err := kube.AppsV1().Deployments("").List(listOpts)
	if err != nil {
		logrus.Errorf("Error fetching Deployments %v", err)
		return nil, err
	}
	for _, item := range deploys.Items {
		var containers []ContainerResult
		for _, container := range item.Spec.Template.Spec.Containers {
			containers = append(containers, formatContainer(container, corev1.ContainerStatus{}, item.Spec.Template.GetCreationTimestamp()))

		}
		deployment := formatControllers("Deployment", item.Name, item.Namespace, string(item.UID), item.GetObjectMeta().GetOwnerReferences(), containers, item.Annotations, item.Labels)
		interfaces = append(interfaces, deployment)
	}

	// Statefulsets
	statefulSets, err := kube.AppsV1().StatefulSets("").List(listOpts)
	if err != nil {
		logrus.Errorf("Error fetching StatefulSets%v", err)
		return nil, err
	}
	for _, item := range statefulSets.Items {
		var containers []ContainerResult
		for _, container := range item.Spec.Template.Spec.Containers {
			containers = append(containers, formatContainer(container, corev1.ContainerStatus{}, item.Spec.Template.GetCreationTimestamp()))
		}
		statefulset := formatControllers("StatefulSet", item.Name, item.Namespace, string(item.UID), item.GetObjectMeta().GetOwnerReferences(), containers, item.Annotations, item.Labels)
		interfaces = append(interfaces, statefulset)
	}

	// DaemonSets
	daemonSets, err := kube.AppsV1().DaemonSets("").List(listOpts)
	if err != nil {
		logrus.Errorf("Error fetching DaemonSets %v", err)
		return nil, err
	}

	for _, item := range daemonSets.Items {
		var containers []ContainerResult
		for _, container := range item.Spec.Template.Spec.Containers {
			containers = append(containers, formatContainer(container, corev1.ContainerStatus{}, item.Spec.Template.GetCreationTimestamp()))
		}
		daemonSet := formatControllers("DaemonSet", item.Name, item.Namespace, string(item.UID), item.GetObjectMeta().GetOwnerReferences(), containers, item.Annotations, item.Labels)
		interfaces = append(interfaces, daemonSet)
	}

	// Jobs
	jobs, err := kube.BatchV1().Jobs("").List(listOpts)
	if err != nil {
		logrus.Errorf("Error fetching Jobs %v", err)
		return nil, err
	}

	for _, item := range jobs.Items {
		var containers []ContainerResult
		for _, container := range item.Spec.Template.Spec.Containers {
			containers = append(containers, formatContainer(container, corev1.ContainerStatus{}, item.Spec.Template.GetCreationTimestamp()))
		}
		job := formatControllers("Job", item.Name, item.Namespace, string(item.UID), item.GetObjectMeta().GetOwnerReferences(), containers, item.Annotations, item.Labels)
		interfaces = append(interfaces, job)
	}

	// CronJobs
	cronJobs, err := kube.BatchV1beta1().CronJobs("").List(listOpts)
	if err != nil {
		logrus.Errorf("Error fetching CronJobs %v", err)
		return nil, err
	}

	for _, item := range cronJobs.Items {
		var containers []ContainerResult
		for _, container := range item.Spec.JobTemplate.Spec.Template.Spec.Containers {
			containers = append(containers, formatContainer(container, corev1.ContainerStatus{}, item.Spec.JobTemplate.GetCreationTimestamp()))
		}
		job := formatControllers("CronJob", item.Name, item.Namespace, string(item.UID), item.GetObjectMeta().GetOwnerReferences(), containers, item.Annotations, item.Labels)
		interfaces = append(interfaces, job)
	}

	// ReplicationControllers
	replicationControllers, err := kube.CoreV1().ReplicationControllers("").List(listOpts)
	if err != nil {
		logrus.Errorf("Error fetching ReplicationControllers %v", err)
		return nil, err
	}

	for _, item := range replicationControllers.Items {
		var containers []ContainerResult
		for _, container := range item.Spec.Template.Spec.Containers {
			containers = append(containers, formatContainer(container, corev1.ContainerStatus{}, item.Spec.Template.GetCreationTimestamp()))
		}
		replicationController := formatControllers("ReplicationController", item.Name, item.Namespace, string(item.UID), item.GetObjectMeta().GetOwnerReferences(), containers, item.Annotations, item.Labels)
		interfaces = append(interfaces, replicationController)
	}

	// ReplicaSet
	replicationSetControllers, err := kube.AppsV1().ReplicaSets("").List(listOpts)
	if err != nil {
		logrus.Errorf("Error fetching ReplicaSets %v", err)
		return nil, err
	}

	for _, item := range replicationSetControllers.Items {
		var containers []ContainerResult
		for _, container := range item.Spec.Template.Spec.Containers {
			containers = append(containers, formatContainer(container, corev1.ContainerStatus{}, item.Spec.Template.GetCreationTimestamp()))
		}
		replicationController := formatControllers("ReplicaSet", item.Name, item.Namespace, string(item.UID), item.GetObjectMeta().GetOwnerReferences(), containers, item.Annotations, item.Labels)
		interfaces = append(interfaces, replicationController)
	}

	// Nodes
	nodes, err := kube.CoreV1().Nodes().List(listOpts)
	if err != nil {
		logrus.Errorf("Error fetching Nodes %v", err)
		return nil, err
	}

	nodesSummaries := make([]NodeSummary, 0)

	for _, item := range nodes.Items {
		node := NodeSummary{
			Name:              item.GetName(),
			Labels:            item.GetLabels(),
			Annotations:       item.GetAnnotations(),
			CreationTimestamp: item.GetCreationTimestamp().UTC(),
			Capacity:          item.Status.Capacity,
			Allocatable:       item.Status.Allocatable,
			KubeletVersion:    item.Status.NodeInfo.KubeletVersion,
			KubeProxyVersion:  item.Status.NodeInfo.KubeProxyVersion,
		}
		allocated, utilization, err := GetNodeAllocatedResource(kube, item)
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
	namespaces, err := kube.CoreV1().Namespaces().List(listOpts)
	if err != nil {
		logrus.Errorf("Error fetching Namespaces %v", err)
		return nil, err
	}

	// Pods
	pods, err := kube.CoreV1().Pods("").List(listOpts)
	if err != nil {
		logrus.Errorf("Error fetching Pods %v", err)
		return nil, err
	}

	for _, item := range pods.Items {
		var containers []ContainerResult
		for _, container := range item.Spec.Containers {
			for _, status := range item.Status.ContainerStatuses {
				if status.Name == container.Name {
					containers = append(containers, formatContainer(container, status, item.CreationTimestamp))
					break
				}
			}
		}
		pod := formatControllers("Pod", item.Name, item.Namespace, string(item.UID), item.GetObjectMeta().GetOwnerReferences(), containers, item.Annotations, item.Labels)
		interfaces = append(interfaces, pod)
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

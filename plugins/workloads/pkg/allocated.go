package workloads

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	k8sClient "k8s.io/client-go/kubernetes"
)

// This file is heavily adapted from
// https://github.com/kubernetes/dashboard/blob/master/src/app/backend/resource/node/detail.go

// NodeAllocatedResources describes node allocated resources.
type NodeAllocatedResources struct {
	// Requests is the total requested resources of all pods in the node
	Requests v1.ResourceList
	// Requests is the total resource limits of all pods in the node
	Limits v1.ResourceList
}

type NodeUtilization struct {
	// CPURequestsFraction is a fraction of CPU, that is allocated.
	CPURequestsFraction float64 `json:"cpuRequestsFraction"`

	// CPULimitsFraction is a fraction of defined CPU limit, can be over 100%, i.e.
	// overcommitted.
	CPULimitsFraction float64 `json:"cpuLimitsFraction"`

	// MemoryRequestsFraction is a fraction of memory, that is allocated.
	MemoryRequestsFraction float64 `json:"memoryRequestsFraction"`

	// MemoryLimitsFraction is a fraction of defined memory limit, can be over 100%, i.e.
	// overcommitted.
	MemoryLimitsFraction float64 `json:"memoryLimitsFraction"`

	// GPURequestsFraction is a fraction of GPU, that is allocated.
	GPURequestsFraction float64 `json:"gpuRequestsFraction"`

	// GPULimitsFraction is a fraction of defined GPU limit, can be over 100%, i.e.
	// overcommitted.
	GPULimitsFraction float64 `json:"gpuLimitsFraction"`
}

// gpuResourceNames lists all supported GPU resource types across vendors
var gpuResourceNames = []v1.ResourceName{
	"nvidia.com/gpu",
	"nvidia.com/gpu.shared",
	"amd.com/gpu",
	"intel.com/gpu",
	"habana.ai/gaudi",
	"google.com/tpu",
	"k8s.amazonaws.com/vgpu",
}

func GetNodeAllocatedResource(ctx context.Context, client k8sClient.Interface, node v1.Node) (NodeAllocatedResources, NodeUtilization, error) {
	pods, err := getNodePods(ctx, client, node)
	if err != nil {
		return NodeAllocatedResources{}, NodeUtilization{}, err
	}
	return getNodeAllocatedResources(node, pods)
}

func getNodePods(ctx context.Context, client k8sClient.Interface, node v1.Node) (*v1.PodList, error) {
	fieldSelector, err := fields.ParseSelector("spec.nodeName=" + node.Name +
		",status.phase!=" + string(v1.PodSucceeded) +
		",status.phase!=" + string(v1.PodFailed))

	if err != nil {
		return nil, err
	}

	return client.CoreV1().Pods(v1.NamespaceAll).List(ctx, metaV1.ListOptions{
		FieldSelector: fieldSelector.String(),
	})
}

func getNodeAllocatedResources(node v1.Node, podList *v1.PodList) (NodeAllocatedResources, NodeUtilization, error) {
	reqs, limits := v1.ResourceList{}, v1.ResourceList{}

	for _, pod := range podList.Items {
		podReqs, podLimits, err := PodRequestsAndLimits(&pod)
		if err != nil {
			return NodeAllocatedResources{}, NodeUtilization{}, err
		}
		for podReqName, podReqValue := range podReqs {
			if value, ok := reqs[podReqName]; !ok {
				reqs[podReqName] = podReqValue.DeepCopy()
			} else {
				value.Add(podReqValue)
				reqs[podReqName] = value
			}
		}
		for podLimitName, podLimitValue := range podLimits {
			if value, ok := limits[podLimitName]; !ok {
				limits[podLimitName] = podLimitValue.DeepCopy()
			} else {
				value.Add(podLimitValue)
				limits[podLimitName] = value
			}
		}
	}
	reqs[v1.ResourcePods] = *resource.NewQuantity(int64(len(podList.Items)), "")
	limits[v1.ResourcePods] = *resource.NewQuantity(int64(len(podList.Items)), "")

	cpuRequests, cpuLimits, memoryRequests, memoryLimits := reqs[v1.ResourceCPU],
		limits[v1.ResourceCPU], reqs[v1.ResourceMemory], limits[v1.ResourceMemory]

	var cpuRequestsFraction, cpuLimitsFraction float64 = 0, 0
	if capacity := float64(node.Status.Capacity.Cpu().MilliValue()); capacity > 0 {
		cpuRequestsFraction = float64(cpuRequests.MilliValue()) / capacity
		cpuLimitsFraction = float64(cpuLimits.MilliValue()) / capacity
	}

	var memoryRequestsFraction, memoryLimitsFraction float64 = 0, 0
	if capacity := float64(node.Status.Capacity.Memory().MilliValue()); capacity > 0 {
		memoryRequestsFraction = float64(memoryRequests.MilliValue()) / capacity
		memoryLimitsFraction = float64(memoryLimits.MilliValue()) / capacity
	}

	// GPU utilization calculation - aggregate across all GPU resource types
	// A node may have multiple GPU types (e.g., nvidia.com/gpu + nvidia.com/gpu.shared)
	var totalGPUCapacity, totalGPURequests, totalGPULimits float64 = 0, 0, 0
	for _, gpuResource := range gpuResourceNames {
		if capacity := node.Status.Capacity[gpuResource]; !capacity.IsZero() {
			totalGPUCapacity += float64(capacity.Value())
			if gpuReqs, ok := reqs[gpuResource]; ok {
				totalGPURequests += float64(gpuReqs.Value())
			}
			if gpuLimits, ok := limits[gpuResource]; ok {
				totalGPULimits += float64(gpuLimits.Value())
			}
		}
	}
	var gpuRequestsFraction, gpuLimitsFraction float64 = 0, 0
	if totalGPUCapacity > 0 {
		gpuRequestsFraction = totalGPURequests / totalGPUCapacity
		gpuLimitsFraction = totalGPULimits / totalGPUCapacity
	}

	return NodeAllocatedResources{
			Requests: reqs,
			Limits:   limits,
		}, NodeUtilization{
			CPURequestsFraction:    cpuRequestsFraction,
			CPULimitsFraction:      cpuLimitsFraction,
			MemoryRequestsFraction: memoryRequestsFraction,
			MemoryLimitsFraction:   memoryLimitsFraction,
			GPURequestsFraction:    gpuRequestsFraction,
			GPULimitsFraction:      gpuLimitsFraction,
		}, nil
}

// PodRequestsAndLimits returns a dictionary of all defined resources summed up for all
// containers of the pod. If pod overhead is non-nil, the pod overhead is added to the
// total container resource requests and to the total container limits which have a
// non-zero quantity.
func PodRequestsAndLimits(pod *v1.Pod) (reqs, limits v1.ResourceList, err error) {
	reqs, limits = v1.ResourceList{}, v1.ResourceList{}
	for _, container := range pod.Spec.Containers {
		addResourceList(reqs, container.Resources.Requests)
		addResourceList(limits, container.Resources.Limits)
	}
	// init containers define the minimum of any resource
	for _, container := range pod.Spec.InitContainers {
		maxResourceList(reqs, container.Resources.Requests)
		maxResourceList(limits, container.Resources.Limits)
	}
	return
}

// addResourceList adds the resources in newList to list
func addResourceList(list, newResourceList v1.ResourceList) {
	for name, quantity := range newResourceList {
		if value, ok := list[name]; !ok {
			list[name] = quantity.DeepCopy()
		} else {
			value.Add(quantity)
			list[name] = value
		}
	}
}

// maxResourceList sets list to the greater of list/newList for every resource
// either list
func maxResourceList(list, newResourceList v1.ResourceList) {
	for name, quantity := range newResourceList {
		if value, ok := list[name]; !ok {
			list[name] = quantity.DeepCopy()
			continue
		} else {
			if quantity.Cmp(value) > 0 {
				list[name] = quantity.DeepCopy()
			}
		}
	}
}

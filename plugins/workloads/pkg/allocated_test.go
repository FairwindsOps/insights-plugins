package workloads

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetNodeAllocatedResources_NoGPU(t *testing.T) {
	node := v1.Node{
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("8"),
				v1.ResourceMemory: resource.MustParse("32Gi"),
			},
		},
	}

	podList := &v1.PodList{
		Items: []v1.Pod{
			{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("1"),
									v1.ResourceMemory: resource.MustParse("1Gi"),
								},
								Limits: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("2"),
									v1.ResourceMemory: resource.MustParse("2Gi"),
								},
							},
						},
					},
				},
			},
		},
	}

	_, utilization, err := getNodeAllocatedResources(node, podList)

	assert.NoError(t, err)
	assert.Equal(t, float64(0), utilization.GPURequestsFraction)
	assert.Equal(t, float64(0), utilization.GPULimitsFraction)
}

func TestGetNodeAllocatedResources_SingleGPUType(t *testing.T) {
	node := v1.Node{
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourceCPU:              resource.MustParse("8"),
				v1.ResourceMemory:           resource.MustParse("32Gi"),
				v1.ResourceName("nvidia.com/gpu"): resource.MustParse("4"),
			},
		},
	}

	podList := &v1.PodList{
		Items: []v1.Pod{
			{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:                    resource.MustParse("1"),
									v1.ResourceName("nvidia.com/gpu"): resource.MustParse("2"),
								},
								Limits: v1.ResourceList{
									v1.ResourceCPU:                    resource.MustParse("2"),
									v1.ResourceName("nvidia.com/gpu"): resource.MustParse("2"),
								},
							},
						},
					},
				},
			},
		},
	}

	allocated, utilization, err := getNodeAllocatedResources(node, podList)

	assert.NoError(t, err)
	// 2 GPUs requested out of 4 = 0.5
	assert.Equal(t, float64(0.5), utilization.GPURequestsFraction)
	assert.Equal(t, float64(0.5), utilization.GPULimitsFraction)
	// Verify raw values are also in AllocatedRequests/Limits
	gpuReqs := allocated.Requests[v1.ResourceName("nvidia.com/gpu")]
	gpuLimits := allocated.Limits[v1.ResourceName("nvidia.com/gpu")]
	assert.Equal(t, int64(2), (&gpuReqs).Value())
	assert.Equal(t, int64(2), (&gpuLimits).Value())
}

func TestGetNodeAllocatedResources_MultipleGPUTypes(t *testing.T) {
	node := v1.Node{
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourceCPU:                        resource.MustParse("8"),
				v1.ResourceMemory:                     resource.MustParse("32Gi"),
				v1.ResourceName("nvidia.com/gpu"):        resource.MustParse("4"),
				v1.ResourceName("nvidia.com/gpu.shared"): resource.MustParse("8"),
			},
		},
	}

	podList := &v1.PodList{
		Items: []v1.Pod{
			{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceName("nvidia.com/gpu"): resource.MustParse("2"),
								},
								Limits: v1.ResourceList{
									v1.ResourceName("nvidia.com/gpu"): resource.MustParse("2"),
								},
							},
						},
					},
				},
			},
			{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceName("nvidia.com/gpu.shared"): resource.MustParse("4"),
								},
								Limits: v1.ResourceList{
									v1.ResourceName("nvidia.com/gpu.shared"): resource.MustParse("4"),
								},
							},
						},
					},
				},
			},
		},
	}

	_, utilization, err := getNodeAllocatedResources(node, podList)

	assert.NoError(t, err)
	// Total capacity: 4 + 8 = 12
	// Total requests: 2 + 4 = 6
	// Fraction: 6/12 = 0.5
	assert.Equal(t, float64(0.5), utilization.GPURequestsFraction)
	assert.Equal(t, float64(0.5), utilization.GPULimitsFraction)
}

func TestGetNodeAllocatedResources_GPUOvercommitted(t *testing.T) {
	node := v1.Node{
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourceCPU:              resource.MustParse("8"),
				v1.ResourceMemory:           resource.MustParse("32Gi"),
				v1.ResourceName("nvidia.com/gpu"): resource.MustParse("2"),
			},
		},
	}

	// Multiple pods requesting more GPUs than available (theoretical overcommit)
	podList := &v1.PodList{
		Items: []v1.Pod{
			{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceName("nvidia.com/gpu"): resource.MustParse("2"),
								},
								Limits: v1.ResourceList{
									v1.ResourceName("nvidia.com/gpu"): resource.MustParse("3"),
								},
							},
						},
					},
				},
			},
		},
	}

	_, utilization, err := getNodeAllocatedResources(node, podList)

	assert.NoError(t, err)
	// 2 requested / 2 capacity = 1.0 (100%)
	assert.Equal(t, float64(1.0), utilization.GPURequestsFraction)
	// 3 limit / 2 capacity = 1.5 (150% overcommitted)
	assert.Equal(t, float64(1.5), utilization.GPULimitsFraction)
}

func TestGetNodeAllocatedResources_AMDAndIntelGPU(t *testing.T) {
	node := v1.Node{
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourceCPU:              resource.MustParse("8"),
				v1.ResourceMemory:           resource.MustParse("32Gi"),
				v1.ResourceName("amd.com/gpu"):   resource.MustParse("2"),
				v1.ResourceName("intel.com/gpu"): resource.MustParse("2"),
			},
		},
	}

	podList := &v1.PodList{
		Items: []v1.Pod{
			{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceName("amd.com/gpu"): resource.MustParse("1"),
								},
								Limits: v1.ResourceList{
									v1.ResourceName("amd.com/gpu"): resource.MustParse("1"),
								},
							},
						},
					},
				},
			},
			{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceName("intel.com/gpu"): resource.MustParse("1"),
								},
								Limits: v1.ResourceList{
									v1.ResourceName("intel.com/gpu"): resource.MustParse("1"),
								},
							},
						},
					},
				},
			},
		},
	}

	allocated, utilization, err := getNodeAllocatedResources(node, podList)

	assert.NoError(t, err)
	// Total capacity: 2 + 2 = 4
	// Total requests: 1 + 1 = 2
	// Fraction: 2/4 = 0.5
	assert.Equal(t, float64(0.5), utilization.GPURequestsFraction)
	assert.Equal(t, float64(0.5), utilization.GPULimitsFraction)
	// Verify individual GPU types are preserved in ResourceList
	amdGPU := allocated.Requests[v1.ResourceName("amd.com/gpu")]
	intelGPU := allocated.Requests[v1.ResourceName("intel.com/gpu")]
	assert.Equal(t, int64(1), (&amdGPU).Value())
	assert.Equal(t, int64(1), (&intelGPU).Value())
}

func TestGetNodeAllocatedResources_GPUWithNoPodsScheduled(t *testing.T) {
	node := v1.Node{
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourceCPU:              resource.MustParse("8"),
				v1.ResourceMemory:           resource.MustParse("32Gi"),
				v1.ResourceName("nvidia.com/gpu"): resource.MustParse("4"),
			},
		},
	}

	podList := &v1.PodList{Items: []v1.Pod{}}

	_, utilization, err := getNodeAllocatedResources(node, podList)

	assert.NoError(t, err)
	assert.Equal(t, float64(0), utilization.GPURequestsFraction)
	assert.Equal(t, float64(0), utilization.GPULimitsFraction)
}

func TestPodRequestsAndLimits_IncludesGPU(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gpu-pod",
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name: "container1",
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:                    resource.MustParse("1"),
							v1.ResourceMemory:                 resource.MustParse("1Gi"),
							v1.ResourceName("nvidia.com/gpu"): resource.MustParse("2"),
						},
						Limits: v1.ResourceList{
							v1.ResourceCPU:                    resource.MustParse("2"),
							v1.ResourceMemory:                 resource.MustParse("2Gi"),
							v1.ResourceName("nvidia.com/gpu"): resource.MustParse("2"),
						},
					},
				},
				{
					Name: "container2",
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceName("nvidia.com/gpu"): resource.MustParse("1"),
						},
						Limits: v1.ResourceList{
							v1.ResourceName("nvidia.com/gpu"): resource.MustParse("1"),
						},
					},
				},
			},
		},
	}

	reqs, limits, err := PodRequestsAndLimits(pod)

	assert.NoError(t, err)
	// GPU requests should be summed across containers: 2 + 1 = 3
	gpuReqs := reqs[v1.ResourceName("nvidia.com/gpu")]
	gpuLimits := limits[v1.ResourceName("nvidia.com/gpu")]
	assert.Equal(t, int64(3), (&gpuReqs).Value())
	assert.Equal(t, int64(3), (&gpuLimits).Value())
}

func TestGPUResourceNames_ContainsAllVendors(t *testing.T) {
	expectedResources := []v1.ResourceName{
		"nvidia.com/gpu",
		"nvidia.com/gpu.shared",
		"amd.com/gpu",
		"intel.com/gpu",
		"habana.ai/gaudi",
		"google.com/tpu",
		"k8s.amazonaws.com/vgpu",
	}

	assert.Equal(t, expectedResources, gpuResourceNames)
	assert.Len(t, gpuResourceNames, 7)
}

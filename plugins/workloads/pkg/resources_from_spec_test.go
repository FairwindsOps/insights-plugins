package workloads

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestResourcesFromContainerSpec_includesGPUMaps(t *testing.T) {
	ctr := corev1.Container{
		Name:  "app",
		Image: "img",
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:                    resource.MustParse("100m"),
				corev1.ResourceMemory:                 resource.MustParse("64Mi"),
				corev1.ResourceName("nvidia.com/gpu"): resource.MustParse("1"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceName("nvidia.com/gpu"): resource.MustParse("1"),
			},
		},
	}
	res := resourcesFromContainerSpec(ctr)
	require.Equal(t, map[string]string{"nvidia.com/gpu": "1"}, res.GPURequests)
	require.Equal(t, map[string]string{"nvidia.com/gpu": "1"}, res.GPULimits)
}

func TestResourcesFromContainerSpec_noGPU(t *testing.T) {
	ctr := corev1.Container{
		Name:  "app",
		Image: "img",
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
		},
	}
	res := resourcesFromContainerSpec(ctr)
	require.Nil(t, res.GPURequests)
	require.Nil(t, res.GPULimits)
}

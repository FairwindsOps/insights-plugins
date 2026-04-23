package workloads

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// kubeStateMetricsGPUResourceToken matches kube-state-metrics label mangling for
// container resource names (see prometheus plugin gpuResourcePattern).
func kubeStateMetricsGPUResourceToken(apiForm string) string {
	return strings.ReplaceAll(strings.ReplaceAll(apiForm, "/", "_"), ".", "_")
}

func TestGPUClassResourceNamesAlignWithPrometheusPlugin(t *testing.T) {
	// Copied from plugins/prometheus/pkg/data/prometheus.go (gpuResourcePattern); if this
	// fails, update gpuClassResourceNameSet and/or prometheus together.
	const promGPUResourcePattern = `nvidia_com_gpu|nvidia_com_gpu_shared|k8s_amazonaws_com_vgpu|amd_com_gpu|intel_com_gpu|habana_ai_gaudi|google_com_tpu`
	want := strings.Split(promGPUResourcePattern, "|")
	sort.Strings(want)

	got := make([]string, 0, len(gpuClassResourceNameSet))
	for n := range gpuClassResourceNameSet {
		got = append(got, kubeStateMetricsGPUResourceToken(string(n)))
	}
	sort.Strings(got)

	require.Equal(t, want, got, "keep gpuClassResourceNameSet and plugins/prometheus/pkg/data/prometheus.go gpuResourcePattern in sync")
}

func TestIsGPUClassResourceName(t *testing.T) {
	require.True(t, isGPUClassResourceName("nvidia.com/gpu"))
	require.True(t, isGPUClassResourceName("nvidia.com/gpu.shared"))
	require.True(t, isGPUClassResourceName("k8s.amazonaws.com/vgpu"))
	require.True(t, isGPUClassResourceName("amd.com/gpu"))
	require.False(t, isGPUClassResourceName("cpu"))
	require.False(t, isGPUClassResourceName("memory"))
	require.False(t, isGPUClassResourceName("ephemeral-storage"))
}

func TestExtendedGPUMapsFromResourceRequirements(t *testing.T) {
	req := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"nvidia.com/gpu": resource.MustParse("2"),
		},
		Limits: corev1.ResourceList{
			"nvidia.com/gpu": resource.MustParse("2"),
		},
	}
	rq, lim := extendedGPUMapsFromResourceRequirements(req)
	require.Equal(t, map[string]string{"nvidia.com/gpu": "2"}, rq)
	require.Equal(t, map[string]string{"nvidia.com/gpu": "2"}, lim)
}

func TestExtendedGPUMaps_requestFromLimitMirror(t *testing.T) {
	req := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits: corev1.ResourceList{
			"amd.com/gpu": resource.MustParse("1"),
		},
	}
	rq, lim := extendedGPUMapsFromResourceRequirements(req)
	require.Equal(t, map[string]string{"amd.com/gpu": "1"}, rq)
	require.Equal(t, map[string]string{"amd.com/gpu": "1"}, lim)
}

func TestGpuClassResourceNamesInUnion(t *testing.T) {
	spec := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"nvidia.com/gpu": resource.MustParse("1"),
		},
	}
	status := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"amd.com/gpu": resource.MustParse("1"),
		},
	}
	names := gpuClassResourceNamesInUnion(spec, status)
	require.Equal(t, []corev1.ResourceName{"amd.com/gpu", "nvidia.com/gpu"}, names)
}

package workloads

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func kubeStateMetricsGPUResourceToken(apiForm string) string {
	return strings.ReplaceAll(strings.ReplaceAll(apiForm, "/", "_"), ".", "_")
}

func TestGPUClassResourceNamesAlignWithPrometheusPlugin(t *testing.T) {
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
	require.False(t, isGPUClassResourceName("nvidia.com/mig-1g.5gb"))
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

func TestExtendedGPUMapsFromResourceRequirements_edgeCases(t *testing.T) {
	cases := []struct {
		name    string
		req     *corev1.ResourceRequirements
		wantReq map[string]string
		wantLim map[string]string
	}{
		{
			name:    "nil",
			req:     nil,
			wantReq: nil,
			wantLim: nil,
		},
		{
			name:    "empty_requests_and_limits",
			req:     &corev1.ResourceRequirements{},
			wantReq: nil,
			wantLim: nil,
		},
		{
			name: "request_without_limit",
			req: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"nvidia.com/gpu": resource.MustParse("1"),
				},
			},
			wantReq: map[string]string{"nvidia.com/gpu": "1"},
			wantLim: nil,
		},
		{
			name: "multiple_gpu_types",
			req: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"nvidia.com/gpu": resource.MustParse("1"),
					"intel.com/gpu":  resource.MustParse("2"),
				},
				Limits: corev1.ResourceList{
					"nvidia.com/gpu": resource.MustParse("1"),
					"intel.com/gpu":  resource.MustParse("2"),
				},
			},
			wantReq: map[string]string{"intel.com/gpu": "2", "nvidia.com/gpu": "1"},
			wantLim: map[string]string{"intel.com/gpu": "2", "nvidia.com/gpu": "1"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotReq, gotLim := extendedGPUMapsFromResourceRequirements(tc.req)
			require.Equal(t, tc.wantReq, gotReq)
			require.Equal(t, tc.wantLim, gotLim)
		})
	}
}

func TestGPUClassResourceNamesInUnion(t *testing.T) {
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

func TestGPUClassResourceNamesInUnion_edgeCases(t *testing.T) {
	cases := []struct {
		name         string
		spec, status *corev1.ResourceRequirements
		want         []corev1.ResourceName
	}{
		{
			name: "limits_only_in_spec",
			spec: &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					"nvidia.com/gpu": resource.MustParse("1"),
				},
			},
			want: []corev1.ResourceName{"nvidia.com/gpu"},
		},
		{
			name: "status_limits_only",
			spec: &corev1.ResourceRequirements{},
			status: &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					"amd.com/gpu": resource.MustParse("2"),
				},
			},
			want: []corev1.ResourceName{"amd.com/gpu"},
		},
		{
			name: "non_gpu_resources_ignored",
			spec: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
					"nvidia.com/gpu":      resource.MustParse("1"),
				},
			},
			want: []corev1.ResourceName{"nvidia.com/gpu"},
		},
		{
			name: "dedup_when_same_gpu_in_spec_requests_and_status_limits",
			spec: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"nvidia.com/gpu": resource.MustParse("1"),
				},
			},
			status: &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					"nvidia.com/gpu": resource.MustParse("1"),
				},
			},
			want: []corev1.ResourceName{"nvidia.com/gpu"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := gpuClassResourceNamesInUnion(tc.spec, tc.status)
			require.Equal(t, tc.want, got)
		})
	}
}

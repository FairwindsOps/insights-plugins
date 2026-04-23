package workloads

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
)

// isGPUClassResourceName returns true for extended resource names that the Fairwinds
// prometheus collector treats as GPU/accelerator requests/limits (see
// plugins/prometheus/pkg/data/prometheus.go gpuResourcePattern). Kubernetes API
// uses dotted names; kube-state-metrics labels use underscores — here we match API names.
func isGPUClassResourceName(name corev1.ResourceName) bool {
	switch string(name) {
	case "nvidia.com/gpu", "nvidia.com/gpu.shared",
		"k8s.amazonaws.com/vgpu",
		"amd.com/gpu",
		"intel.com/gpu",
		"habana.ai/gaudi",
		"google.com/tpu":
		return true
	default:
		return false
	}
}

// gpuClassResourceNamesInUnion returns sorted unique GPU-class resource names present
// in either spec or status requests or limits.
func gpuClassResourceNamesInUnion(spec, status *corev1.ResourceRequirements) []corev1.ResourceName {
	seen := map[corev1.ResourceName]struct{}{}
	add := func(rl corev1.ResourceList) {
		for n := range rl {
			if isGPUClassResourceName(n) {
				seen[n] = struct{}{}
			}
		}
	}
	if spec != nil {
		add(spec.Requests)
		add(spec.Limits)
	}
	if status != nil {
		add(status.Requests)
		add(status.Limits)
	}
	out := make([]corev1.ResourceName, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// gpuClassResourceNamesInRequirements returns sorted unique GPU-class names in a single
// ResourceRequirements (used when building applied extended maps from status only).
func gpuClassResourceNamesInRequirements(req *corev1.ResourceRequirements) []corev1.ResourceName {
	if req == nil {
		return nil
	}
	return gpuClassResourceNamesInUnion(req, nil)
}

func resourceQuantityString(rl corev1.ResourceList, name corev1.ResourceName) string {
	if q, ok := rl[name]; ok {
		return q.String()
	}
	return ""
}

// extendedGPUMapsFromResourceRequirements returns GPU-class request and limit quantity
// strings from a ResourceRequirements (container spec or applied status). When a GPU
// request is unset but the limit is set, the request map entry mirrors the limit (same
// rule as CPU/memory on the main Requests/Limits fields).
func extendedGPUMapsFromResourceRequirements(req *corev1.ResourceRequirements) (requests, limits map[string]string) {
	if req == nil {
		return nil, nil
	}
	if len(req.Requests) == 0 && len(req.Limits) == 0 {
		return nil, nil
	}
	extReq := map[string]string{}
	extLim := map[string]string{}
	for _, n := range gpuClassResourceNamesInRequirements(req) {
		key := string(n)
		reqStr := resourceQuantityString(req.Requests, n)
		limStr := resourceQuantityString(req.Limits, n)
		if reqStr == "" && limStr != "" {
			reqStr = limStr
		}
		if reqStr != "" {
			extReq[key] = reqStr
		}
		if limStr != "" {
			extLim[key] = limStr
		}
	}
	if len(extReq) == 0 {
		extReq = nil
	}
	if len(extLim) == 0 {
		extLim = nil
	}
	return extReq, extLim
}

package workloads

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// aggregateAppliedResourcesByContainer returns per-container applied resources
// derived from pod status (status.containerStatuses[].resources), using only
// Running pods that report Ready. When multiple pods disagree, the majority wins;
// on a tie at the top, Actual is omitted (nil) for that container.
func aggregateAppliedResourcesByContainer(podObjs []unstructured.Unstructured, podSpec *corev1.PodSpec) map[string]*AppliedResources {
	if podSpec == nil || len(podObjs) == 0 {
		return nil
	}

	out := make(map[string]*AppliedResources)
	for _, c := range podSpec.Containers {
		if ar := aggregateOneContainer(c.Name, podObjs); ar != nil {
			out[c.Name] = ar
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func aggregateOneContainer(containerName string, podObjs []unstructured.Unstructured) *AppliedResources {
	counts := map[string]int{}
	canonical := map[string]AppliedResources{}

	for _, u := range podObjs {
		pod := &corev1.Pod{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), pod); err != nil {
			continue
		}
		if pod.Status.Phase != corev1.PodRunning || !podReady(pod) {
			continue
		}
		var cs *corev1.ContainerStatus
		for i := range pod.Status.ContainerStatuses {
			if pod.Status.ContainerStatuses[i].Name == containerName {
				cs = &pod.Status.ContainerStatuses[i]
				break
			}
		}
		if cs == nil || cs.Resources == nil || !resourceRequirementsPopulated(cs.Resources) {
			continue
		}
		applied := appliedResourcesFromRequirements(cs.Resources)
		if applied == nil {
			continue
		}
		k := appliedResourcesKey(*applied)
		counts[k]++
		canonical[k] = *applied
	}
	if len(counts) == 0 {
		return nil
	}

	maxVotes := 0
	for _, c := range counts {
		if c > maxVotes {
			maxVotes = c
		}
	}
	var winners []string
	for k, c := range counts {
		if c == maxVotes {
			winners = append(winners, k)
		}
	}
	if len(winners) != 1 {
		return nil
	}
	w := canonical[winners[0]]
	return &w
}

func podReady(pod *corev1.Pod) bool {
	for i := range pod.Status.Conditions {
		c := pod.Status.Conditions[i]
		if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func resourceRequirementsPopulated(req *corev1.ResourceRequirements) bool {
	if req == nil {
		return false
	}
	return len(req.Requests) > 0 || len(req.Limits) > 0
}

// appliedResourcesFromRequirements mirrors the CPU/memory string rules used for spec
// resources (limit copies to request when request is unset).
func appliedResourcesFromRequirements(req *corev1.ResourceRequirements) *AppliedResources {
	if req == nil || !resourceRequirementsPopulated(req) {
		return nil
	}
	out := AppliedResources{
		Requests: ResourcesInfo{
			CPU:    qtyString(req.Requests, corev1.ResourceCPU),
			Memory: qtyString(req.Requests, corev1.ResourceMemory),
		},
		Limits: ResourcesInfo{
			CPU:    qtyString(req.Limits, corev1.ResourceCPU),
			Memory: qtyString(req.Limits, corev1.ResourceMemory),
		},
	}
	cpuReq := req.Requests[corev1.ResourceCPU]
	memReq := req.Requests[corev1.ResourceMemory]
	cpuLim := req.Limits[corev1.ResourceCPU]
	memLim := req.Limits[corev1.ResourceMemory]
	if cpuReq.IsZero() && !cpuLim.IsZero() {
		out.Requests.CPU = out.Limits.CPU
	}
	if memReq.IsZero() && !memLim.IsZero() {
		out.Requests.Memory = out.Limits.Memory
	}
	return &out
}

func qtyString(rl corev1.ResourceList, name corev1.ResourceName) string {
	if q, ok := rl[name]; ok {
		return (&q).String()
	}
	z := resource.MustParse("0")
	return (&z).String()
}

func appliedResourcesKey(a AppliedResources) string {
	return a.Requests.CPU + "\x00" + a.Requests.Memory + "\x00" + a.Limits.CPU + "\x00" + a.Limits.Memory
}

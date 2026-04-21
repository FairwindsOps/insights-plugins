package workloads

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// SpecAppliedSkewPod is one pod whose applied resources (pod status) differ from
// that pod's container spec for the given template container name.
type SpecAppliedSkewPod struct {
	Namespace string
	Name      string
	UID       string
	Applied   AppliedResources
}

// SpecAppliedStats counts Running+Ready pods where status.containerStatuses[].resources
// is present and semantically matches the pod's spec for that container, and lists pods
// where it does not (in-place skew, rollout, etc.).
type SpecAppliedStats struct {
	ConvergedCount int
	SkewPods       []SpecAppliedSkewPod `json:",omitempty"`
}

func computeSpecAppliedStats(containerName string, podObjs []unstructured.Unstructured) SpecAppliedStats {
	var out SpecAppliedStats
	if len(podObjs) == 0 {
		return out
	}

	for _, u := range podObjs {
		pod := &corev1.Pod{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), pod); err != nil {
			continue
		}
		if pod.Status.Phase != corev1.PodRunning || !podReady(pod) {
			continue
		}

		ctnSpec := findContainerInPodSpec(pod, containerName)
		if ctnSpec == nil {
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

		if resourceRequirementsCPUAndMemoryEqual(&ctnSpec.Resources, cs.Resources) {
			out.ConvergedCount++
			continue
		}

		applied := appliedResourcesFromRequirements(cs.Resources)
		if applied == nil {
			continue
		}
		out.SkewPods = append(out.SkewPods, SpecAppliedSkewPod{
			Namespace: pod.Namespace,
			Name:      pod.Name,
			UID:       string(pod.UID),
			Applied:   *applied,
		})
	}

	if len(out.SkewPods) > 0 {
		sort.Slice(out.SkewPods, func(i, j int) bool {
			if out.SkewPods[i].Namespace != out.SkewPods[j].Namespace {
				return out.SkewPods[i].Namespace < out.SkewPods[j].Namespace
			}
			return out.SkewPods[i].Name < out.SkewPods[j].Name
		})
	} else {
		out.SkewPods = nil
	}

	return out
}

func findContainerInPodSpec(pod *corev1.Pod, name string) *corev1.Container {
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == name {
			return &pod.Spec.Containers[i]
		}
	}
	return nil
}

func resourceRequirementsCPUAndMemoryEqual(spec, status *corev1.ResourceRequirements) bool {
	if spec == nil {
		spec = &corev1.ResourceRequirements{}
	}
	if status == nil {
		return false
	}
	return qtyListEqual(spec.Requests, status.Requests, corev1.ResourceCPU) &&
		qtyListEqual(spec.Requests, status.Requests, corev1.ResourceMemory) &&
		qtyListEqual(spec.Limits, status.Limits, corev1.ResourceCPU) &&
		qtyListEqual(spec.Limits, status.Limits, corev1.ResourceMemory)
}

func qtyListEqual(a, b corev1.ResourceList, name corev1.ResourceName) bool {
	var qa, qb resource.Quantity
	if q, ok := a[name]; ok {
		qa = q
	}
	if q, ok := b[name]; ok {
		qb = q
	}
	return qa.Cmp(qb) == 0
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
	return ""
}

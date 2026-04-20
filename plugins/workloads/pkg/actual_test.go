package workloads

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func podUnstructured(t *testing.T, pod *corev1.Pod) unstructured.Unstructured {
	t.Helper()
	o, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pod)
	require.NoError(t, err)
	return unstructured.Unstructured{Object: o}
}

func podWithResources(name, ns string, specReq, statusReq *corev1.ResourceRequirements) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: ns, UID: types.UID("uid-" + name),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:      "app",
				Image:     "img",
				Resources: derefReq(specReq),
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:      "app",
				Resources: statusReq,
			}},
		},
	}
}

func derefReq(r *corev1.ResourceRequirements) corev1.ResourceRequirements {
	if r == nil {
		return corev1.ResourceRequirements{}
	}
	return *r
}

func TestComputeSpecAppliedStats_specMatchesStatus(t *testing.T) {
	res := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("200m"),
		},
	}
	p := podWithResources("p1", "default", res, res)
	u := podUnstructured(t, p)
	stats := computeSpecAppliedStats("app", []unstructured.Unstructured{u})
	require.Equal(t, 1, stats.ConvergedCount)
	require.Nil(t, stats.SkewPods)
}

func TestComputeSpecAppliedStats_skew(t *testing.T) {
	spec := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
	}
	status := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")},
	}
	p := podWithResources("p1", "prod", spec, status)
	u := podUnstructured(t, p)
	stats := computeSpecAppliedStats("app", []unstructured.Unstructured{u})
	require.Equal(t, 0, stats.ConvergedCount)
	require.Len(t, stats.SkewPods, 1)
	require.Equal(t, "prod", stats.SkewPods[0].Namespace)
	require.Equal(t, "p1", stats.SkewPods[0].Name)
	require.Equal(t, "uid-p1", stats.SkewPods[0].UID)
	require.Equal(t, "200m", stats.SkewPods[0].Applied.Requests.CPU)
}

func TestComputeSpecAppliedStats_convergedAndSkew(t *testing.T) {
	res := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
	}
	skewed := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
	}
	p1 := podWithResources("a", "ns", res, res)
	p2 := podWithResources("b", "ns", res, res)
	p3 := podWithResources("c", "ns", res, skewed)
	var u []unstructured.Unstructured
	u = append(u, podUnstructured(t, p1), podUnstructured(t, p2), podUnstructured(t, p3))
	stats := computeSpecAppliedStats("app", u)
	require.Equal(t, 2, stats.ConvergedCount)
	require.Len(t, stats.SkewPods, 1)
	require.Equal(t, "c", stats.SkewPods[0].Name)
}

func TestComputeSpecAppliedStats_semanticCpuEqual(t *testing.T) {
	spec := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
	}
	status := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("0.1")},
	}
	p := podWithResources("p1", "default", spec, status)
	u := podUnstructured(t, p)
	stats := computeSpecAppliedStats("app", []unstructured.Unstructured{u})
	require.Equal(t, 1, stats.ConvergedCount)
	require.Nil(t, stats.SkewPods)
}

func TestComputeSpecAppliedStats_skipsNotRunning(t *testing.T) {
	res := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
	}
	p := podWithResources("p1", "default", res, res)
	p.Status.Phase = corev1.PodPending
	u := podUnstructured(t, p)
	stats := computeSpecAppliedStats("app", []unstructured.Unstructured{u})
	require.Equal(t, 0, stats.ConvergedCount)
	require.Nil(t, stats.SkewPods)
}

func TestComputeSpecAppliedStats_skipsNotReady(t *testing.T) {
	res := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
	}
	p := podWithResources("p1", "default", res, res)
	p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}}
	u := podUnstructured(t, p)
	stats := computeSpecAppliedStats("app", []unstructured.Unstructured{u})
	require.Equal(t, 0, stats.ConvergedCount)
}

func TestComputeSpecAppliedStats_noStatusResources(t *testing.T) {
	res := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
	}
	p := podWithResources("p1", "default", res, res)
	p.Status.ContainerStatuses[0].Resources = nil
	u := podUnstructured(t, p)
	stats := computeSpecAppliedStats("app", []unstructured.Unstructured{u})
	require.Equal(t, 0, stats.ConvergedCount)
	require.Nil(t, stats.SkewPods)
}

func TestAppliedResourcesFromRequirements_requestFromLimitMirror(t *testing.T) {
	req := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("500m"),
		},
	}
	out := appliedResourcesFromRequirements(req)
	require.NotNil(t, out)
	require.Equal(t, "500m", out.Requests.CPU)
	require.Equal(t, "500m", out.Limits.CPU)
}

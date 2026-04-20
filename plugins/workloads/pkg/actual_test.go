package workloads

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func podUnstructured(t *testing.T, pod *corev1.Pod) unstructured.Unstructured {
	t.Helper()
	o, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pod)
	require.NoError(t, err)
	return unstructured.Unstructured{Object: o}
}

func readyPod(name string, res *corev1.ResourceRequirements) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "app", Resources: res},
			},
		},
	}
}

func TestAggregateAppliedResourcesByContainer_nilAndEmpty(t *testing.T) {
	spec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "app", Image: "x"}},
	}
	require.Nil(t, aggregateAppliedResourcesByContainer(nil, spec))
	require.Nil(t, aggregateAppliedResourcesByContainer(nil, nil))

	out := aggregateAppliedResourcesByContainer([]unstructured.Unstructured{}, spec)
	require.Nil(t, out)
}

func TestAggregateAppliedResourcesByContainer_singlePod(t *testing.T) {
	spec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "app", Image: "x"}},
	}
	res := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
	}
	u := podUnstructured(t, readyPod("p1", res))
	out := aggregateAppliedResourcesByContainer([]unstructured.Unstructured{u}, spec)
	require.NotNil(t, out)
	require.NotNil(t, out["app"])
	require.Equal(t, "100m", out["app"].Requests.CPU)
	require.Equal(t, "128Mi", out["app"].Requests.Memory)
	require.Equal(t, "200m", out["app"].Limits.CPU)
	require.Equal(t, "256Mi", out["app"].Limits.Memory)
}

func TestAggregateAppliedResourcesByContainer_majorityTie(t *testing.T) {
	spec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "app", Image: "x"}},
	}
	resA := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
	}
	resB := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")},
	}
	p1 := podUnstructured(t, readyPod("p1", resA))
	p2 := podUnstructured(t, readyPod("p2", resB))
	out := aggregateAppliedResourcesByContainer([]unstructured.Unstructured{p1, p2}, spec)
	require.Nil(t, out)
}

func TestAggregateAppliedResourcesByContainer_majorityWins(t *testing.T) {
	spec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "app", Image: "x"}},
	}
	resA := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
	}
	resB := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")},
	}
	p1 := podUnstructured(t, readyPod("p1", resA))
	p2 := podUnstructured(t, readyPod("p2", resA))
	p3 := podUnstructured(t, readyPod("p3", resB))
	out := aggregateAppliedResourcesByContainer([]unstructured.Unstructured{p1, p2, p3}, spec)
	require.NotNil(t, out)
	require.NotNil(t, out["app"])
	require.Equal(t, "100m", out["app"].Requests.CPU)
}

func TestAggregateAppliedResourcesByContainer_skipsNotRunning(t *testing.T) {
	spec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "app", Image: "x"}},
	}
	res := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
	}
	pod := readyPod("p1", res)
	pod.Status.Phase = corev1.PodPending
	u := podUnstructured(t, pod)
	out := aggregateAppliedResourcesByContainer([]unstructured.Unstructured{u}, spec)
	require.Nil(t, out)
}

func TestAggregateAppliedResourcesByContainer_skipsNotReady(t *testing.T) {
	spec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "app", Image: "x"}},
	}
	res := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
	}
	pod := readyPod("p1", res)
	pod.Status.Conditions = []corev1.PodCondition{
		{Type: corev1.PodReady, Status: corev1.ConditionFalse},
	}
	u := podUnstructured(t, pod)
	out := aggregateAppliedResourcesByContainer([]unstructured.Unstructured{u}, spec)
	require.Nil(t, out)
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

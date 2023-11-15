package data

import (
	"fmt"
	"testing"
	"time"

	"github.com/fairwindsops/controller-utils/pkg/controller"
	pluginmodels "github.com/fairwindsops/insights-plugins/plugins/prometheus/pkg/models"
	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetKey(t *testing.T) {
	sample := model.SampleStream{
		Metric: model.Metric{
			"namespace": "ns",
			"pod":       "pd",
			"container": "cont",
		},
	}
	assert.Equal(t, "ns/pd/cont", getKey(&sample))
}

func TestGetOwner(t *testing.T) {
	sample := model.SampleStream{
		Metric: model.Metric{
			"namespace": "ns",
			"pod":       "pd",
			"container": "cont",
		},
	}
	owner := getOwner(&sample)
	assert.Equal(t, "ns", owner.ControllerNamespace)
	assert.Equal(t, "pd", owner.PodName)
	assert.Equal(t, "cont", owner.Container)
}

func TestGetController(t *testing.T) {
	podName := "asdf-1234567-abcde"
	namespace := "default"
	workloads := []controller.Workload{
		{
			TopController: unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Deployment",
					"metadata": map[string]interface{}{
						"name":      "asdf",
						"namespace": "default",
					},
					"spec": map[string]interface{}{},
				},
			},
			Pods: []unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"kind": "Pod",
						"metadata": map[string]interface{}{
							"name":      "asdf-1234567-abcde",
							"namespace": "default",
						},
						"spec": map[string]interface{}{},
					},
				},
			},
		},
		{
			TopController: unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Job",
					"metadata": map[string]interface{}{
						"name":      "asdf",
						"namespace": "default",
					},
					"spec": map[string]interface{}{},
				},
			},
			Pods: []unstructured.Unstructured{},
		},
		{
			TopController: unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "ReplicaSet",
					"metadata": map[string]interface{}{
						"name":      "asdf2-12a3468",
						"namespace": "default",
					},
					"spec": map[string]interface{}{},
				},
			},
			Pods: []unstructured.Unstructured{},
		},
	}
	name, kind := getController(workloads, podName, namespace)
	assert.Equal(t, "asdf", name)
	assert.Equal(t, "Deployment", kind)

	podName = "foobar-default"
	name, kind = getController(workloads, podName, namespace)
	assert.Equal(t, podName, name)
	assert.Equal(t, "Pod", kind)

	podName = "asdf2-12a3468-asd1e"
	name, kind = getController(workloads, podName, namespace)
	assert.Equal(t, "asdf2-12a3468", name)
	assert.Equal(t, "ReplicaSet", kind)
}

func TestAdjustMetricsForMultiContainerPods(t *testing.T) {
	testMetrics := []*model.SampleStream{
		{
			Metric: model.Metric{
				"namespace": "default",
				"pod":       "testpod",
			},
			Values: []model.SamplePair{
				{
					Timestamp: 1674153900000,
					Value:     0.5,
				},
				{
					Timestamp: 1674153930000,
					Value:     1.5,
				},
				{
					Timestamp: 1674153960000,
					Value:     0.0,
				},
				{
					Timestamp: 1674153990000,
					Value:     5.3,
				},
				{
					Timestamp: 1674154020000,
					Value:     9.0,
				},
				{
					Timestamp: 1674154050000,
					Value:     10.0,
				},
			},
		},
	}

	// This minimal workload information is used to determine the number of
	// containers and their names, matched to the above testMetrics by the
	// "namespace/name" of the pod.
	workloads := make(map[string]*controller.Workload)
	workloads["default/testpod"] = &controller.Workload{
		PodSpec: &corev1.PodSpec{
			// Define the minimal information we need for each container, its name.
			Containers: []corev1.Container{
				{
					Name: "container1",
				},
				{
					Name: "container2",
				},
				{
					Name: "container3",
				},
			},
		},
	}

	adjustedMetrics := adjustMetricsForMultiContainerPods(testMetrics, workloads)
	assert.Equal(t, len(adjustedMetrics)-len(testMetrics), 2, "number of new metrics after splitting them across containers of multi-pod containers")
	wantMetrics := []*model.SampleStream{
		{
			Metric: model.Metric{
				"namespace": "default",
				"pod":       "testpod",
				"container": "container1",
			},
			Values: []model.SamplePair{
				{
					Timestamp: 1674153900000,
					Value:     0.5,
				},
				{
					Timestamp: 1674153930000,
					Value:     1.5,
				},
				{
					Timestamp: 1674153960000,
					Value:     0.0,
				},
				{
					Timestamp: 1674153990000,
					Value:     3.3,
				},
				{
					Timestamp: 1674154020000,
					Value:     3.0,
				},
				{
					Timestamp: 1674154050000,
					Value:     4.0,
				},
			},
		},
		{
			Metric: model.Metric{
				"namespace": "default",
				"pod":       "testpod",
				"container": "container2",
			},
			Values: []model.SamplePair{
				{
					Timestamp: 1674153900000,
					Value:     0.0,
				},
				{
					Timestamp: 1674153930000,
					Value:     0.0,
				},
				{
					Timestamp: 1674153960000,
					Value:     0.0,
				},
				{
					Timestamp: 1674153990000,
					Value:     1.0,
				},
				{
					Timestamp: 1674154020000,
					Value:     3.0,
				},
				{
					Timestamp: 1674154050000,
					Value:     3.0,
				},
			},
		},
		{
			Metric: model.Metric{
				"namespace": "default",
				"pod":       "testpod",
				"container": "container3",
			},
			Values: []model.SamplePair{
				{
					Timestamp: 1674153900000,
					Value:     0.0,
				},
				{
					Timestamp: 1674153930000,
					Value:     0.0,
				},
				{
					Timestamp: 1674153960000,
					Value:     0.0,
				},
				{
					Timestamp: 1674153990000,
					Value:     1.0,
				},
				{
					Timestamp: 1674154020000,
					Value:     3.0,
				},
				{
					Timestamp: 1674154050000,
					Value:     3.0,
				},
			},
		},
	}
	// Assert individual struct fields of each adjusted metric, for readability
	// and easier troubleshooting.
	for i := 0; i <= len(wantMetrics)-1; i++ {
		assert.EqualValues(t, wantMetrics[i].Metric, adjustedMetrics[i].Metric, fmt.Sprintf("metric map for index %d of adjustedMetrics", i))
		assert.EqualValues(t, wantMetrics[i].Values, adjustedMetrics[i].Values, fmt.Sprintf("values for index %d (container %s) of adjustedMetrics", i, adjustedMetrics[i].Metric["container"]))
	}
}

// TestStorageCapacity( verifies a PersistentVolumeClaim shared by multiple
// pods is correctly divided by the number of pods, and that a multi-container
// pod is also divided by container.
func TestStorageCapacity(t *testing.T) {
	// TODO: This test is failing
	unstructuredPVCs := []unstructured.Unstructured{ // Minimal required fields for a test PersistentVolumeClaim.
		{
			Object: map[string]interface{}{
				"kind": "PersistentVolumeClaim",
				"metadata": map[string]interface{}{
					"name": "pvc1",
				},
				"spec": map[string]interface{}{},
				"status": map[string]interface{}{
					"capacity": map[string]interface{}{
						"storage": "8Gi",
					},
				},
			},
		},
	}
	storageInfo := pluginmodels.NewStorageInfoFromUnstructuredPVCs(unstructuredPVCs)
	// Associatepods with the above PVC.
	storageInfo.AddPVCRef("pvc1", "default/pod1")
	storageInfo.AddPVCRef("pvc1", "default/pod2")
	storageInfo.AddPVCRef("pvc1", "default/pod3")
	// This minimal workload information is used to determine the number of
	// containers and their names, matched to the above unstructuredPVCs by the
	// "namespace/name" of the pod that references the PVC.
	workloads := make(map[string]*controller.Workload)
	workloads["default/pod1"] = &controller.Workload{
		PodSpec: &corev1.PodSpec{
			// Define the minimal information we need for each container, its name.
			Containers: []corev1.Container{
				{
					Name: "container1forpod1",
				},
				{
					Name: "container2forpod1",
				},
				{
					Name: "container3forpod1",
				},
			},
		},
	}
	workloads["default/pod2"] = &controller.Workload{
		PodSpec: &corev1.PodSpec{
			// Define the minimal information we need for each container, its name.
			Containers: []corev1.Container{
				{
					Name: "container1forpod2",
				},
				{
					Name: "container2forpod2",
				},
				{
					Name: "container3forpod2",
				},
			},
		},
	}
	workloads["default/pod3"] = &controller.Workload{
		PodSpec: &corev1.PodSpec{
			// Define the minimal information we need for each container, its name.
			Containers: []corev1.Container{
				{
					Name: "container1forpod3",
				},
				{
					Name: "container2forpod3",
				},
				{
					Name: "container3forpod3",
				},
			},
		},
	}

	// This prometheus range represents our typical metric collection.
	r := prometheusV1.Range{
		Start: time.Date(2023, time.January, 19, 18, 45, 0, 0, time.UTC),
		End:   time.Date(2023, time.January, 19, 19, 0, 0, 0, time.UTC),
		Step:  30000000000,
	}

	metrics := storageInfo.ManufactureMetrics(r)
	assert.Equal(t, 3, len(metrics), "number of per-pod metrics created from PersistentVolumeClaims")
	adjustedMetrics := adjustMetricsForMultiContainerPods(metrics, workloads)
	assert.Equal(t, 9, len(adjustedMetrics), "number of metrics after adjusting per-pod metrics to be per-container")

	// The first 3 metrics should be for the containers of pod1.
	// Dynamically construct the expected container name (container1...,
	// container2...) using the index `i`.
	for i := 0; i <= 2; i++ {
		assert.Equal(t,
			model.Metric{
				"namespace": "default",
				"pod":       "pod1",
				"container": model.LabelValue(fmt.Sprintf("container%dforpod1", i+1)), // container name "containerXforpod1"
			},
			adjustedMetrics[i].Metric,
			fmt.Sprintf("labels for adjusted metric at index %d include the correct container name", i))
	}

	// Metrics at index 3-5 should be for the containers of pod2.
	// Dynamically construct the expected container name (container1...,
	// container2...) using the index `i`.
	for i := 3; i <= 5; i++ {
		assert.Equal(t,
			model.Metric{
				"namespace": "default",
				"pod":       "pod2",
				"container": model.LabelValue(fmt.Sprintf("container%dforpod2", i-2)), // container name "containerXforpod2"
			},
			adjustedMetrics[i].Metric,
			fmt.Sprintf("labels for adjusted metric at index %d include the correct container name", i))
	}

	// Metrics at index 6-8 should be for the containers of pod3.
	// Dynamically construct the expected container name (container1...,
	// container2...) using the index `i`.
	for i := 6; i <= 8; i++ {
		assert.Equal(t,
			model.Metric{
				"namespace": "default",
				"pod":       "pod3",
				"container": model.LabelValue(fmt.Sprintf("container%dforpod3", i-5)), // container name "containerXforpod2"
			},
			adjustedMetrics[i].Metric,
			fmt.Sprintf("labels for adjusted metric at index %d include the correct container name", i))
	}

	// Verify the capacity of the shared PVC was split up correctly
	// Verify the capacity of the shared PVC was split up correctly per-pod and
	// per-metric.
	// Although the below only asserts values for a single pod (pod1), the
	// correctness of the metric values does implicitly validate that the PVC has
	// been split correctly across the pods that share the PVC.
	// Additionally, all values of a single metric do not need to be checked, because PVC
	// metrics use the same value for all time-stamps.
	// Note that adjustedMetrics[0] has a slightly larger value than
	// adjustedMetrics[1] or ...[2], because the first metric value holds the
	// remainder of the divided value.
	assert.Equal(t, model.SampleValue(954437178.0), adjustedMetrics[0].Values[0].Value, "the metric value for pod1 and container name container1forpod1")
	assert.Equal(t, model.SampleValue(954437177.0), adjustedMetrics[1].Values[0].Value, "the metric value for pod1 and container name container2forpod1")
	assert.Equal(t, model.SampleValue(954437177.0), adjustedMetrics[2].Values[0].Value, "the metric value for pod1 and container name container3forpod1")
}

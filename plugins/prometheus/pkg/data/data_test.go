package data

import (
	"fmt"
	"testing"
	"time"

	"github.com/fairwindsops/controller-utils/pkg/controller"
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

func TestConvertingCumulativeValuesToDeltaValues(t *testing.T) {
	// This prometheus range represents our typical metric collection.
	r := prometheusV1.Range{
		Start: time.Date(2023, time.January, 19, 18, 45, 0, 0, time.UTC),
		End:   time.Date(2023, time.January, 19, 19, 0, 0, 0, time.UTC),
		Step:  30000000000,
	}
	// This slice of samples includes 2 elements whos time falls outside of the
	// above prometheus range. These extra values are used later to obtain a baseline value
	// when converting totals to deltas.
	v := []model.SamplePair{
		model.SamplePair{Timestamp: 1674153840000, Value: 5},   // TImestamp begins before r.Start
		model.SamplePair{Timestamp: 1674153870000, Value: 6},   // Timestamp begins before r.Start
		model.SamplePair{Timestamp: 1674153900000, Value: 6.5}, // timestamp is 18:45:00 UTC
		model.SamplePair{Timestamp: 1674153930000, Value: 7},
		model.SamplePair{Timestamp: 1674153960000, Value: 7},
		model.SamplePair{Timestamp: 1674153990000, Value: 7},
		model.SamplePair{Timestamp: 1674154020000, Value: 7},
		model.SamplePair{Timestamp: 1674154050000, Value: 7},
		model.SamplePair{Timestamp: 1674154080000, Value: 7},
		model.SamplePair{Timestamp: 1674154110000, Value: 7},
		model.SamplePair{Timestamp: 1674154140000, Value: 7},
		model.SamplePair{Timestamp: 1674154170000, Value: 7},
		model.SamplePair{Timestamp: 1674154200000, Value: 7},
		model.SamplePair{Timestamp: 1674154230000, Value: 7},
		model.SamplePair{Timestamp: 1674154260000, Value: 7},
		model.SamplePair{Timestamp: 1674154290000, Value: 7},
		model.SamplePair{Timestamp: 1674154320000, Value: 7},
		model.SamplePair{Timestamp: 1674154350000, Value: 7},
		model.SamplePair{Timestamp: 1674154380000, Value: 7},
		model.SamplePair{Timestamp: 1674154410000, Value: 7},
		// TImestamp skips 18:54:00 through 18:55:30
		model.SamplePair{Timestamp: 1674154560000, Value: 7},
		model.SamplePair{Timestamp: 1674154590000, Value: 7},
		model.SamplePair{Timestamp: 1674154620000, Value: 7},
		model.SamplePair{Timestamp: 1674154650000, Value: 7},
		model.SamplePair{Timestamp: 1674154680000, Value: 7},
		model.SamplePair{Timestamp: 1674154710000, Value: 7},
		model.SamplePair{Timestamp: 1674154740000, Value: 7},
		model.SamplePair{Timestamp: 1674154770000, Value: 7},
		model.SamplePair{Timestamp: 1674154800000, Value: 7}, // timestamp is 19:00:00 UTC
	}

	t.Logf("values before convertion from totals to delta are: %#v", v)
	assert.Equal(t, len(v), 29, "number of prometheus values before conversion")
	newV, err := cumulitiveValuesToDeltaValues(v, r)
	assert.NoError(t, err)
	t.Logf("values after conversion are: %#v", newV)
	assert.Equal(t, len(newV), 27, "number of prometheus values after conversion")
	wantV := []model.SamplePair{model.SamplePair{Timestamp: 1674153900000, Value: 0.5}, model.SamplePair{Timestamp: 1674153930000, Value: 0.5}, model.SamplePair{Timestamp: 1674153960000, Value: 0}, model.SamplePair{Timestamp: 1674153990000, Value: 0}, model.SamplePair{Timestamp: 1674154020000, Value: 0}, model.SamplePair{Timestamp: 1674154050000, Value: 0}, model.SamplePair{Timestamp: 1674154080000, Value: 0}, model.SamplePair{Timestamp: 1674154110000, Value: 0}, model.SamplePair{Timestamp: 1674154140000, Value: 0}, model.SamplePair{Timestamp: 1674154170000, Value: 0}, model.SamplePair{Timestamp: 1674154200000, Value: 0}, model.SamplePair{Timestamp: 1674154230000, Value: 0}, model.SamplePair{Timestamp: 1674154260000, Value: 0}, model.SamplePair{Timestamp: 1674154290000, Value: 0}, model.SamplePair{Timestamp: 1674154320000, Value: 0}, model.SamplePair{Timestamp: 1674154350000, Value: 0}, model.SamplePair{Timestamp: 1674154380000, Value: 0}, model.SamplePair{Timestamp: 1674154410000, Value: 0}, model.SamplePair{Timestamp: 1674154560000, Value: 0}, model.SamplePair{Timestamp: 1674154590000, Value: 0}, model.SamplePair{Timestamp: 1674154620000, Value: 0}, model.SamplePair{Timestamp: 1674154650000, Value: 0}, model.SamplePair{Timestamp: 1674154680000, Value: 0}, model.SamplePair{Timestamp: 1674154710000, Value: 0}, model.SamplePair{Timestamp: 1674154740000, Value: 0}, model.SamplePair{Timestamp: 1674154770000, Value: 0}, model.SamplePair{Timestamp: 1674154800000, Value: 0}}
	assert.Equal(t, wantV, newV)
}

func TestAdjustMetricsForMultiContainerPods(t *testing.T) {
	testMetrics := []*model.SampleStream{
		&model.SampleStream{
			Metric: model.Metric{
				"namespace": "default",
				"pod":       "testpod",
			},
			Values: []model.SamplePair{
				model.SamplePair{
					Timestamp: 1674153900000,
					Value:     0.5,
				},
				model.SamplePair{
					Timestamp: 1674153930000,
					Value:     1.5,
				},
				model.SamplePair{
					Timestamp: 1674153960000,
					Value:     0.0,
				},
				model.SamplePair{
					Timestamp: 1674153990000,
					Value:     5.3,
				},
				model.SamplePair{
					Timestamp: 1674154020000,
					Value:     9.0,
				},
				model.SamplePair{
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
		&model.SampleStream{
			Metric: model.Metric{
				"namespace": "default",
				"pod":       "testpod",
				"container": "container1",
			},
			Values: []model.SamplePair{
				model.SamplePair{
					Timestamp: 1674153900000,
					Value:     0.5,
				},
				model.SamplePair{
					Timestamp: 1674153930000,
					Value:     1.5,
				},
				model.SamplePair{
					Timestamp: 1674153960000,
					Value:     0.0,
				},
				model.SamplePair{
					Timestamp: 1674153990000,
					Value:     3.3,
				},
				model.SamplePair{
					Timestamp: 1674154020000,
					Value:     3.0,
				},
				model.SamplePair{
					Timestamp: 1674154050000,
					Value:     4.0,
				},
			},
		},
		&model.SampleStream{
			Metric: model.Metric{
				"namespace": "default",
				"pod":       "testpod",
				"container": "container2",
			},
			Values: []model.SamplePair{
				model.SamplePair{
					Timestamp: 1674153900000,
					Value:     0.0,
				},
				model.SamplePair{
					Timestamp: 1674153930000,
					Value:     0.0,
				},
				model.SamplePair{
					Timestamp: 1674153960000,
					Value:     0.0,
				},
				model.SamplePair{
					Timestamp: 1674153990000,
					Value:     1.0,
				},
				model.SamplePair{
					Timestamp: 1674154020000,
					Value:     3.0,
				},
				model.SamplePair{
					Timestamp: 1674154050000,
					Value:     3.0,
				},
			},
		},
		&model.SampleStream{
			Metric: model.Metric{
				"namespace": "default",
				"pod":       "testpod",
				"container": "container3",
			},
			Values: []model.SamplePair{
				model.SamplePair{
					Timestamp: 1674153900000,
					Value:     0.0,
				},
				model.SamplePair{
					Timestamp: 1674153930000,
					Value:     0.0,
				},
				model.SamplePair{
					Timestamp: 1674153960000,
					Value:     0.0,
				},
				model.SamplePair{
					Timestamp: 1674153990000,
					Value:     1.0,
				},
				model.SamplePair{
					Timestamp: 1674154020000,
					Value:     3.0,
				},
				model.SamplePair{
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

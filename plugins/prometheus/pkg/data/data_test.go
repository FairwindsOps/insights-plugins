package data

import (
	"testing"

	"github.com/fairwindsops/controller-utils/pkg/controller"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
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
	podName = "asdf-default"

	name, kind = getController(workloads, podName, namespace)
	assert.Equal(t, podName, name)
	assert.Equal(t, "Pod", kind)
	podName = "asdf2-12a3468-asd1e"

	name, kind = getController(workloads, podName, namespace)
	assert.Equal(t, "asdf2-12a3468", name)
	assert.Equal(t, "ReplicaSet", kind)
}

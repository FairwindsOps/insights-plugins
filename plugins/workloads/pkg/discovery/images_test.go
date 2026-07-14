package discovery

import (
	"context"
	"testing"

	fwControllerUtils "github.com/fairwindsops/controller-utils/pkg/controller"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestHasControllerOwner(t *testing.T) {
	trueVal := true
	falseVal := false

	require.True(t, hasControllerOwner([]metav1.OwnerReference{
		{Controller: &trueVal},
	}))
	require.False(t, hasControllerOwner([]metav1.OwnerReference{
		{Controller: &falseVal},
	}))
	require.False(t, hasControllerOwner([]metav1.OwnerReference{
		{Kind: "ReplicaSet", Name: "api-123"},
	}))
	require.False(t, hasControllerOwner(nil))
}

func TestContainerStatusesFromPod(t *testing.T) {
	pod := corev1.Pod{
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "app", Image: "app:1.0.0"},
			},
			InitContainerStatuses: []corev1.ContainerStatus{
				{Name: "init", Image: "init:1.0.0"},
			},
			EphemeralContainerStatuses: []corev1.ContainerStatus{
				{Name: "debug", Image: "debug:1.0.0"},
			},
		},
	}

	statuses := containerStatusesFromPod(pod)
	require.Len(t, statuses, 3)
}

func TestRecordContainerImageDedupesOwners(t *testing.T) {
	keyToImage := map[string]ImageResult{}
	imageOwners := map[string]map[string]OwnerResult{}

	status := corev1.ContainerStatus{
		Name:    "app",
		Image:   "docker.io/library/nginx:1.25",
		ImageID: "docker-pullable://docker.io/library/nginx@sha256:abc",
	}

	owner1 := OwnerResult{Namespace: "prod", Kind: "Deployment", Name: "api"}
	keyToImage, imageOwners = recordContainerImage(status, owner1, keyToImage, imageOwners)

	owner2 := OwnerResult{Namespace: "prod", Kind: "Deployment", Name: "api-backup"}
	keyToImage, imageOwners = recordContainerImage(status, owner2, keyToImage, imageOwners)

	require.Len(t, keyToImage, 1)
	require.Len(t, imageOwners, 1)

	for key, owners := range imageOwners {
		require.Len(t, owners, 2)
		img := keyToImage[key]
		require.Equal(t, "docker.io/library/nginx:1.25", img.Name)
		require.Equal(t, "docker.io/library/nginx@sha256:abc", img.ID)
	}
}

func TestRecordContainerImageStripsDockerPullablePrefix(t *testing.T) {
	keyToImage := map[string]ImageResult{}
	imageOwners := map[string]map[string]OwnerResult{}

	status := corev1.ContainerStatus{
		Name:    "app",
		Image:   "quay.io/fairwinds/goldilocks:v2.2.0",
		ImageID: "docker-pullable://quay.io/fairwinds/goldilocks@sha256:abc123",
	}
	owner := OwnerResult{Namespace: "insights-agent", Kind: "Deployment", Name: "goldilocks"}

	keyToImage, _ = recordContainerImage(status, owner, keyToImage, imageOwners)

	require.Len(t, keyToImage, 1)
	for _, img := range keyToImage {
		require.Equal(t, "quay.io/fairwinds/goldilocks@sha256:abc123", img.ID)
		require.Equal(t, "quay.io/fairwinds/goldilocks@sha256:abc123", img.PullRef)
	}
}

func TestRecordContainerImageUsesImageIDWhenImageIsSha256Digest(t *testing.T) {
	keyToImage := map[string]ImageResult{}
	imageOwners := map[string]map[string]OwnerResult{}

	status := corev1.ContainerStatus{
		Name:    "app",
		Image:   "sha256:deadbeef",
		ImageID: "docker-pullable://registry.example.io/app@sha256:deadbeef",
	}
	owner := OwnerResult{Namespace: "default", Kind: "Deployment", Name: "api"}

	keyToImage, _ = recordContainerImage(status, owner, keyToImage, imageOwners)

	require.Len(t, keyToImage, 1)
	for _, img := range keyToImage {
		require.Equal(t, "registry.example.io/app@sha256:deadbeef", img.Name)
		require.Equal(t, "registry.example.io/app@sha256:deadbeef", img.ID)
	}
}

func TestRecordContainerImageIncludesInitContainer(t *testing.T) {
	pod := corev1.Pod{
		Status: corev1.PodStatus{
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name:    "init",
					Image:   "busybox:1.36",
					ImageID: "docker-pullable://busybox@sha256:init123",
				},
			},
		},
	}

	keyToImage := map[string]ImageResult{}
	imageOwners := map[string]map[string]OwnerResult{}
	owner := OwnerResult{Namespace: "default", Kind: "Pod", Name: "orphan"}

	for _, status := range containerStatusesFromPod(pod) {
		keyToImage, imageOwners = recordContainerImage(status, owner, keyToImage, imageOwners)
	}

	require.Len(t, keyToImage, 1)
	for _, img := range keyToImage {
		require.Equal(t, "busybox:1.36", img.Name)
	}
}

func TestRecordContainerImageSkipsEmptyImageID(t *testing.T) {
	keyToImage := map[string]ImageResult{}
	imageOwners := map[string]map[string]OwnerResult{}

	status := corev1.ContainerStatus{
		Name:  "app",
		Image: "nginx:latest",
	}
	owner := OwnerResult{Namespace: "default", Kind: "Deployment", Name: "api"}

	keyToImage, imageOwners = recordContainerImage(status, owner, keyToImage, imageOwners)

	require.Empty(t, keyToImage)
	require.Empty(t, imageOwners)
}

func TestOrphanPodHasPodOwnerKind(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "standalone",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:    "app",
					Image:   "nginx:1.25",
					ImageID: "docker-pullable://nginx@sha256:orphan",
				},
			},
		},
	}

	require.False(t, hasControllerOwner(pod.OwnerReferences))

	keyToImage := map[string]ImageResult{}
	imageOwners := map[string]map[string]OwnerResult{}
	owner := OwnerResult{
		Namespace: pod.Namespace,
		Kind:      "Pod",
		Name:      pod.Name,
	}
	for _, status := range containerStatusesFromPod(pod) {
		keyToImage, imageOwners = recordContainerImage(status, owner, keyToImage, imageOwners)
	}

	require.Len(t, keyToImage, 1)
	for _, owners := range imageOwners {
		for _, o := range owners {
			require.Equal(t, "Pod", o.Kind)
			require.Equal(t, "standalone", o.Name)
		}
	}
}

func TestFinalizeImagesSortsDeterministically(t *testing.T) {
	keyToImage := map[string]ImageResult{
		"b/b-id": {Name: "b", ID: "b-id"},
		"a/a-id": {Name: "a", ID: "a-id"},
	}
	imageOwners := map[string]map[string]OwnerResult{
		"a/a-id": {
			"prod/Deployment/a/app": {Namespace: "prod", Kind: "Deployment", Name: "a", Container: "app"},
			"prod/Deployment/z/app": {Namespace: "prod", Kind: "Deployment", Name: "z", Container: "app"},
		},
	}

	images := finalizeImages(keyToImage, imageOwners)

	require.Len(t, images, 2)
	require.Equal(t, "a", images[0].Name)
	require.Equal(t, "b", images[1].Name)
	require.Equal(t, "a", images[0].Owners[0].Name)
	require.Equal(t, "z", images[0].Owners[1].Name)
}

func TestListImagesControllerPass(t *testing.T) {
	pod := runningPod("default", "api-abc", corev1.ContainerStatus{
		Name:    "app",
		Image:   "nginx:1.25",
		ImageID: "docker-pullable://nginx@sha256:controller",
	})
	podUnstructured, err := podToUnstructured(pod)
	require.NoError(t, err)

	controllers := []fwControllerUtils.Workload{
		{
			TopController: unstructured.Unstructured{
				Object: map[string]any{
					"kind": "Deployment",
					"metadata": map[string]any{
						"name":      "api",
						"namespace": "default",
						"labels": map[string]any{
							"app": "api",
						},
						"annotations": map[string]any{
							"deployed-by": "ci",
						},
					},
				},
			},
			PodMetadata: &metav1.ObjectMeta{
				Labels: map[string]string{
					"pod-template-hash": "abc123",
				},
				Annotations: map[string]string{
					"config/version": "1",
				},
			},
			Pods: []unstructured.Unstructured{*podUnstructured},
		},
	}

	client := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
	)

	result, err := ListImages(context.Background(), client, controllers)
	require.NoError(t, err)
	require.Len(t, result.Images, 1)
	require.Equal(t, "Deployment", result.Images[0].Owners[0].Kind)
	require.Equal(t, "api", result.Images[0].Owners[0].Name)
	require.Empty(t, result.Images[0].Owners[0].Labels)
	require.Empty(t, result.Images[0].Owners[0].Annotations)
	require.Empty(t, result.Images[0].Owners[0].PodLabels)
	require.Empty(t, result.Images[0].Owners[0].PodAnnotations)
}

func TestListImagesOrphanPodPass(t *testing.T) {
	pod := runningPod("default", "standalone", corev1.ContainerStatus{
		Name:    "app",
		Image:   "nginx:1.25",
		ImageID: "docker-pullable://nginx@sha256:orphan",
	})
	pod.Labels = map[string]string{"app": "standalone"}
	pod.Annotations = map[string]string{"note": "orphan"}

	client := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		pod,
	)

	result, err := ListImages(context.Background(), client, nil)
	require.NoError(t, err)
	require.Len(t, result.Images, 1)
	require.Equal(t, "Pod", result.Images[0].Owners[0].Kind)
	require.Equal(t, "standalone", result.Images[0].Owners[0].Name)
	require.Equal(t, "standalone", result.Images[0].Owners[0].Labels["app"])
	require.Equal(t, "orphan", result.Images[0].Owners[0].Annotations["note"])
}

func TestListImagesActiveJobPass(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "batch",
			Namespace: "default",
		},
		Spec: batchv1.JobSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"job-name": "batch"},
			},
		},
		Status: batchv1.JobStatus{Active: 1},
	}
	trueVal := true
	pod := runningPod("default", "batch-abc", corev1.ContainerStatus{
		Name:    "worker",
		Image:   "worker:1.0",
		ImageID: "docker-pullable://worker@sha256:job",
	})
	pod.Labels = map[string]string{"job-name": "batch"}
	pod.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: "batch/v1",
			Kind:       "Job",
			Name:       "batch",
			Controller: &trueVal,
		},
	}

	client := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		job,
		pod,
	)

	result, err := ListImages(context.Background(), client, nil)
	require.NoError(t, err)
	require.Len(t, result.Images, 1)
	require.Equal(t, "Job", result.Images[0].Owners[0].Kind)
	require.Equal(t, "batch", result.Images[0].Owners[0].Name)
	require.Equal(t, "worker", result.Images[0].Owners[0].Container)
}

func TestPodPhaseContributesImages(t *testing.T) {
	require.True(t, podPhaseContributesImages(corev1.PodRunning, "Deployment"))
	require.False(t, podPhaseContributesImages(corev1.PodSucceeded, "Deployment"))
	require.False(t, podPhaseContributesImages(corev1.PodFailed, "StatefulSet"))
	require.True(t, podPhaseContributesImages(corev1.PodRunning, "CronJob"))
	require.True(t, podPhaseContributesImages(corev1.PodSucceeded, "CronJob"))
	require.True(t, podPhaseContributesImages(corev1.PodFailed, "CronJob"))
	require.True(t, podPhaseContributesImages(corev1.PodSucceeded, "Job"))
	require.True(t, podPhaseContributesImages(corev1.PodPending, "CronJob"))
}

func TestListImagesPrefersPodSpecOverTruncatedStatusImage(t *testing.T) {
	pod := completedPod("insights-agent", "workloads-29732785-x9h2f", corev1.PodSucceeded, corev1.ContainerStatus{
		Name:    "workloads",
		Image:   "quay.io/fairwinds/workloads:2",
		ImageID: "quay.io/fairwinds/workloads@sha256:f27ac6e9a73c92f987b80aa15036ef6944916e9435ac2df59cabdc7f7fe5a5dd",
	})
	pod.Spec.Containers = []corev1.Container{
		{Name: "workloads", Image: "quay.io/fairwinds/workloads:2.10"},
	}
	podUnstructured, err := podToUnstructured(pod)
	require.NoError(t, err)

	controllers := []fwControllerUtils.Workload{
		{
			TopController: unstructured.Unstructured{
				Object: map[string]any{
					"kind": "CronJob",
					"metadata": map[string]any{
						"name":      "workloads",
						"namespace": "insights-agent",
					},
				},
			},
			Pods: []unstructured.Unstructured{*podUnstructured},
		},
	}

	client := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "insights-agent"}},
	)

	result, err := ListImages(context.Background(), client, controllers)
	require.NoError(t, err)
	require.Len(t, result.Images, 1)
	require.Equal(t, "quay.io/fairwinds/workloads:2.10", result.Images[0].Name)
	require.Equal(t, "quay.io/fairwinds/workloads@sha256:f27ac6e9a73c92f987b80aa15036ef6944916e9435ac2df59cabdc7f7fe5a5dd", result.Images[0].ID)
}

func TestListImagesCronJobCompletedPodPass(t *testing.T) {
	pod := completedPod("insights-agent", "trivy-29732745-84kb8", corev1.PodSucceeded, corev1.ContainerStatus{
		Name:    "trivy",
		Image:   "quay.io/fairwinds/trivy:1.0",
		ImageID: "docker-pullable://quay.io/fairwinds/trivy@sha256:trivy",
	})
	podUnstructured, err := podToUnstructured(pod)
	require.NoError(t, err)

	controllers := []fwControllerUtils.Workload{
		{
			TopController: unstructured.Unstructured{
				Object: map[string]any{
					"kind": "CronJob",
					"metadata": map[string]any{
						"name":      "trivy",
						"namespace": "insights-agent",
					},
				},
			},
			Pods: []unstructured.Unstructured{*podUnstructured},
		},
	}

	client := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "insights-agent"}},
	)

	result, err := ListImages(context.Background(), client, controllers)
	require.NoError(t, err)
	require.Len(t, result.Images, 1)
	require.Equal(t, "CronJob", result.Images[0].Owners[0].Kind)
	require.Equal(t, "trivy", result.Images[0].Owners[0].Name)
	require.Equal(t, "insights-agent", result.Images[0].Owners[0].Namespace)
	require.Equal(t, "trivy", result.Images[0].Owners[0].Container)
	require.Empty(t, result.Images[0].Owners[0].Labels)
	require.Equal(t, "quay.io/fairwinds/trivy@sha256:trivy", result.Images[0].ID)
}

func TestListImagesDeploymentIgnoresSucceededPod(t *testing.T) {
	pod := completedPod("default", "api-abc", corev1.PodSucceeded, corev1.ContainerStatus{
		Name:    "app",
		Image:   "nginx:1.25",
		ImageID: "docker-pullable://nginx@sha256:done",
	})
	podUnstructured, err := podToUnstructured(pod)
	require.NoError(t, err)

	controllers := []fwControllerUtils.Workload{
		{
			TopController: unstructured.Unstructured{
				Object: map[string]any{
					"kind": "Deployment",
					"metadata": map[string]any{
						"name":      "api",
						"namespace": "default",
					},
				},
			},
			Pods: []unstructured.Unstructured{*podUnstructured},
		},
	}

	client := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
	)

	result, err := ListImages(context.Background(), client, controllers)
	require.NoError(t, err)
	require.Empty(t, result.Images)
}

func TestListImagesCompletedJobOwnedByCronJob(t *testing.T) {
	trueVal := true
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "polaris-29732742",
			Namespace: "insights-agent",
			Labels:    map[string]string{"app": "polaris"},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "batch/v1",
					Kind:       "CronJob",
					Name:       "polaris",
					Controller: &trueVal,
				},
			},
		},
		Spec: batchv1.JobSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"job-name": "polaris-29732742"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"job-name": "polaris-29732742"},
				},
			},
		},
		Status: batchv1.JobStatus{Succeeded: 1},
	}
	pod := completedPod("insights-agent", "polaris-29732742-mt86f", corev1.PodSucceeded, corev1.ContainerStatus{
		Name:    "polaris",
		Image:   "quay.io/fairwinds/polaris:9.0",
		ImageID: "docker-pullable://quay.io/fairwinds/polaris@sha256:polaris",
	})
	pod.Labels = map[string]string{"job-name": "polaris-29732742"}
	pod.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: "batch/v1",
			Kind:       "Job",
			Name:       "polaris-29732742",
			Controller: &trueVal,
		},
	}

	client := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "insights-agent"}},
		job,
		pod,
	)

	result, err := ListImages(context.Background(), client, nil)
	require.NoError(t, err)
	require.Len(t, result.Images, 1)
	require.Equal(t, "CronJob", result.Images[0].Owners[0].Kind)
	require.Equal(t, "polaris", result.Images[0].Owners[0].Name)
	require.Empty(t, result.Images[0].Owners[0].Labels)
	require.Empty(t, result.Images[0].Owners[0].Annotations)
}

func TestListImagesCompletedStandaloneJob(t *testing.T) {
	trueVal := true
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oneshot",
			Namespace: "default",
			Labels:    map[string]string{"app": "oneshot"},
		},
		Spec: batchv1.JobSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"job-name": "oneshot"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"job-name": "oneshot"},
				},
			},
		},
		Status: batchv1.JobStatus{Failed: 1},
	}
	pod := completedPod("default", "oneshot-xyz", corev1.PodFailed, corev1.ContainerStatus{
		Name:    "worker",
		Image:   "worker:1.0",
		ImageID: "docker-pullable://worker@sha256:fail",
	})
	pod.Labels = map[string]string{"job-name": "oneshot"}
	pod.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: "batch/v1",
			Kind:       "Job",
			Name:       "oneshot",
			Controller: &trueVal,
		},
	}

	client := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		job,
		pod,
	)

	result, err := ListImages(context.Background(), client, nil)
	require.NoError(t, err)
	require.Len(t, result.Images, 1)
	require.Equal(t, "Job", result.Images[0].Owners[0].Kind)
	require.Equal(t, "oneshot", result.Images[0].Owners[0].Name)
	require.Equal(t, "oneshot", result.Images[0].Owners[0].Labels["app"])
}

func runningPod(namespace, name string, status corev1.ContainerStatus) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				status,
			},
		},
	}
}

func completedPod(namespace, name string, phase corev1.PodPhase, status corev1.ContainerStatus) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: corev1.PodStatus{
			Phase: phase,
			ContainerStatuses: []corev1.ContainerStatus{
				status,
			},
		},
	}
}

func podToUnstructured(pod *corev1.Pod) (*unstructured.Unstructured, error) {
	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pod)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: raw}, nil
}

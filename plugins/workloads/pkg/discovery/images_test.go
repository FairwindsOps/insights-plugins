package discovery

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	imageOwners := map[string]map[OwnerResult]struct{}{}

	status := corev1.ContainerStatus{
		Name:    "app",
		Image:   "docker.io/library/nginx:1.25",
		ImageID: "docker-pullable://docker.io/library/nginx@sha256:abc",
	}

	owner1 := OwnerResult{Namespace: "prod", Kind: "Deployment", Name: "api"}
	recordContainerImage(status, owner1, keyToImage, imageOwners)

	owner2 := OwnerResult{Namespace: "prod", Kind: "Deployment", Name: "api-backup"}
	recordContainerImage(status, owner2, keyToImage, imageOwners)

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
	imageOwners := map[string]map[OwnerResult]struct{}{}

	status := corev1.ContainerStatus{
		Name:    "app",
		Image:   "quay.io/fairwinds/goldilocks:v2.2.0",
		ImageID: "docker-pullable://quay.io/fairwinds/goldilocks@sha256:abc123",
	}
	owner := OwnerResult{Namespace: "insights-agent", Kind: "Deployment", Name: "goldilocks"}

	recordContainerImage(status, owner, keyToImage, imageOwners)

	require.Len(t, keyToImage, 1)
	for _, img := range keyToImage {
		require.Equal(t, "quay.io/fairwinds/goldilocks@sha256:abc123", img.ID)
		require.Equal(t, "quay.io/fairwinds/goldilocks@sha256:abc123", img.PullRef)
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
	imageOwners := map[string]map[OwnerResult]struct{}{}
	owner := OwnerResult{Namespace: "default", Kind: "Pod", Name: "orphan"}

	for _, status := range containerStatusesFromPod(pod) {
		recordContainerImage(status, owner, keyToImage, imageOwners)
	}

	require.Len(t, keyToImage, 1)
	for _, img := range keyToImage {
		require.Equal(t, "busybox:1.36", img.Name)
	}
}

func TestRecordContainerImageSkipsEmptyImageID(t *testing.T) {
	keyToImage := map[string]ImageResult{}
	imageOwners := map[string]map[OwnerResult]struct{}{}

	status := corev1.ContainerStatus{
		Name:  "app",
		Image: "nginx:latest",
	}
	owner := OwnerResult{Namespace: "default", Kind: "Deployment", Name: "api"}

	recordContainerImage(status, owner, keyToImage, imageOwners)

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
	imageOwners := map[string]map[OwnerResult]struct{}{}
	owner := OwnerResult{
		Namespace: pod.Namespace,
		Kind:      "Pod",
		Name:      pod.Name,
	}
	for _, status := range containerStatusesFromPod(pod) {
		recordContainerImage(status, owner, keyToImage, imageOwners)
	}

	require.Len(t, keyToImage, 1)
	for _, owners := range imageOwners {
		for o := range owners {
			require.Equal(t, "Pod", o.Kind)
			require.Equal(t, "standalone", o.Name)
		}
	}
}

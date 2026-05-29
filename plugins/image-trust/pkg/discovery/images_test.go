package discovery

import (
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	corev1 "k8s.io/api/core/v1"
)

func TestNamespaceIsBlocked(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		blocklist []string
		allowlist []string
		want      bool
	}{
		{
			name:      "blocked namespace wins",
			namespace: "kube-system",
			blocklist: []string{"kube-system"},
			want:      true,
		},
		{
			name:      "allowlist permits namespace",
			namespace: "prod",
			allowlist: []string{"prod"},
			want:      false,
		},
		{
			name:      "allowlist blocks everything else",
			namespace: "dev",
			allowlist: []string{"prod"},
			want:      true,
		},
		{
			name:      "no lists means allowed",
			namespace: "default",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := namespaceIsBlocked(tt.namespace, tt.blocklist, tt.allowlist)
			if got != tt.want {
				t.Fatalf("namespaceIsBlocked() = %v, want %v", got, tt.want)
			}
		})
	}
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
	if len(statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(statuses))
	}
}

func TestRecordContainerImageDedupes(t *testing.T) {
	owner := models.Resource{
		Namespace: "prod",
		Kind:      "Deployment",
		Name:      "api",
	}
	keyToImage := map[string]models.DiscoveredImage{}
	imageOwners := map[string]map[models.Resource]struct{}{}

	status := corev1.ContainerStatus{
		Name:    "app",
		Image:   "docker.io/library/nginx:1.25",
		ImageID: "docker-pullable://docker.io/library/nginx@sha256:abc",
	}
	recordContainerImage(status, owner, keyToImage, imageOwners)

	if len(keyToImage) != 1 {
		t.Fatalf("expected one image, got %d", len(keyToImage))
	}
	if len(imageOwners) != 1 {
		t.Fatalf("expected one owner map, got %d", len(imageOwners))
	}
}

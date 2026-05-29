package registry

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const dockerConfigSecretType = "kubernetes.io/dockerconfigjson"

// CollectPullSecretConfigs reads dockerconfigjson secrets referenced by discovered workloads.
func CollectPullSecretConfigs(
	ctx context.Context,
	client kubernetes.Interface,
	refs []PullSecretRef,
) ([]dockerConfig, error) {
	configs := make([]dockerConfig, 0, len(refs))
	seen := make(map[PullSecretRef]struct{}, len(refs))

	for _, ref := range refs {
		if ref.Namespace == "" || ref.Name == "" {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}

		secret, err := client.CoreV1().Secrets(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("getting pull secret %s/%s: %w", ref.Namespace, ref.Name, err)
		}
		if secret.Type != dockerConfigSecretType {
			continue
		}
		raw, ok := secret.Data[corev1.DockerConfigJsonKey]
		if !ok || len(raw) == 0 {
			continue
		}
		cfg, err := parseDockerConfig(raw)
		if err != nil {
			return nil, fmt.Errorf("secret %s/%s: %w", ref.Namespace, ref.Name, err)
		}
		configs = append(configs, cfg)
	}
	return configs, nil
}

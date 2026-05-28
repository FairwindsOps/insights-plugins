package registry

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const dockerConfigSecretType = "kubernetes.io/dockerconfigjson"

// CollectPullSecretConfigs reads dockerconfigjson secrets from namespaces in scope.
func CollectPullSecretConfigs(
	ctx context.Context,
	client kubernetes.Interface,
	namespaceAllowlist, namespaceBlocklist []string,
) ([]dockerConfig, error) {
	namespaces, err := listNamespaces(ctx, client, namespaceAllowlist, namespaceBlocklist)
	if err != nil {
		return nil, err
	}

	configs := make([]dockerConfig, 0)
	for _, namespace := range namespaces {
		secrets, err := client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing secrets in namespace %s: %w", namespace, err)
		}
		for _, secret := range secrets.Items {
			if secret.Type != dockerConfigSecretType {
				continue
			}
			raw, ok := secret.Data[corev1.DockerConfigJsonKey]
			if !ok || len(raw) == 0 {
				continue
			}
			cfg, err := parseDockerConfig(raw)
			if err != nil {
				return nil, fmt.Errorf("secret %s/%s: %w", namespace, secret.Name, err)
			}
			configs = append(configs, cfg)
		}
	}
	return configs, nil
}

func listNamespaces(
	ctx context.Context,
	client kubernetes.Interface,
	namespaceAllowlist, namespaceBlocklist []string,
) ([]string, error) {
	if len(namespaceAllowlist) > 0 {
		namespaces := make([]string, 0, len(namespaceAllowlist))
		for _, ns := range namespaceAllowlist {
			if namespaceIsBlocked(strings.ToLower(ns), namespaceBlocklist, namespaceAllowlist) {
				continue
			}
			namespaces = append(namespaces, ns)
		}
		return namespaces, nil
	}

	list, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing namespaces: %w", err)
	}
	namespaces := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		ns := strings.ToLower(item.Name)
		if namespaceIsBlocked(ns, namespaceBlocklist, namespaceAllowlist) {
			continue
		}
		namespaces = append(namespaces, item.Name)
	}
	return namespaces, nil
}

func namespaceIsBlocked(namespace string, namespaceBlocklist, namespaceAllowlist []string) bool {
	for _, blocked := range namespaceBlocklist {
		if namespace == strings.ToLower(blocked) {
			return true
		}
	}
	if len(namespaceAllowlist) == 0 {
		return false
	}
	for _, allowed := range namespaceAllowlist {
		if namespace == strings.ToLower(allowed) {
			return false
		}
	}
	return true
}

package discovery

import (
	"context"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type pullSecretSet map[registry.PullSecretRef]struct{}

func newPullSecretSet() pullSecretSet {
	return pullSecretSet{}
}

func (s pullSecretSet) refs() []registry.PullSecretRef {
	refs := make([]registry.PullSecretRef, 0, len(s))
	for ref := range s {
		refs = append(refs, ref)
	}
	return refs
}

func (s pullSecretSet) add(namespace, name string) {
	if namespace == "" || name == "" {
		return
	}
	s[registry.PullSecretRef{Namespace: namespace, Name: name}] = struct{}{}
}

func collectPullSecretRefs(
	ctx context.Context,
	client kubernetes.Interface,
	pod corev1.Pod,
	refs pullSecretSet,
	serviceAccounts map[string]struct{},
) {
	for _, secret := range pod.Spec.ImagePullSecrets {
		refs.add(pod.Namespace, secret.Name)
	}

	saName := pod.Spec.ServiceAccountName
	if saName == "" {
		saName = "default"
	}
	cacheKey := pod.Namespace + "/" + saName
	if _, ok := serviceAccounts[cacheKey]; ok {
		return
	}
	serviceAccounts[cacheKey] = struct{}{}

	sa, err := client.CoreV1().ServiceAccounts(pod.Namespace).Get(ctx, saName, metav1.GetOptions{})
	if err != nil {
		logrus.Warnf("loading service account %s/%s for imagePullSecrets: %v", pod.Namespace, saName, err)
		return
	}
	for _, secret := range sa.ImagePullSecrets {
		refs.add(pod.Namespace, secret.Name)
	}
}

package registry

import (
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCollectPullSecretConfigsFetchesOnlyReferencedSecrets(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "prod",
				Name:      "workload-pull",
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				corev1.DockerConfigJsonKey: []byte(`{"auths":{"https://ghcr.io":{"username":"workload","password":"secret"}}}`),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "prod",
				Name:      "unrelated-pull",
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				corev1.DockerConfigJsonKey: []byte(`{"auths":{"https://registry.example":{"username":"other","password":"secret"}}}`),
			},
		},
	)

	configs, err := CollectPullSecretConfigs(t.Context(), client, []PullSecretRef{
		{Namespace: "prod", Name: "workload-pull"},
	})
	if err != nil {
		t.Fatalf("CollectPullSecretConfigs() error = %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}

	raw, err := json.Marshal(configs[0])
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if string(raw) == "" {
		t.Fatal("expected non-empty config")
	}
	if _, ok := configs[0].Auths["https://ghcr.io"]; !ok {
		t.Fatalf("expected ghcr.io auth, got %#v", configs[0].Auths)
	}
}

func TestDockerConfigDirKeychainResolve(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := dockerConfig{}.withBasicAuth("https://ghcr.io", "robot", "secret")
	if err := writeDockerConfigDir(dir, cfg); err != nil {
		t.Fatalf("writeDockerConfigDir() error = %v", err)
	}

	keychain := dockerConfigDirKeychain{dir: dir}
	auth, err := keychain.Resolve(authnResource("ghcr.io"))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if auth == nil {
		t.Fatal("expected authenticator")
	}
}

type authnResource string

func (a authnResource) String() string { return string(a) }
func (a authnResource) RegistryStr() string {
	return string(a)
}

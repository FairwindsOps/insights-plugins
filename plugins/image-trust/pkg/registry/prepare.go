package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/config"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

const defaultRegistryAuthHost = "https://index.docker.io/v1/"

// PreparedCredentials holds registry settings and an optional temp directory to remove.
type PreparedCredentials struct {
	Credentials Credentials
	cleanup     func()
}

// Cleanup removes temporary docker config directories created during preparation.
func (p PreparedCredentials) Cleanup() {
	if p.cleanup != nil {
		p.cleanup()
	}
}

// Prepare loads environment credentials and optionally merges imagePullSecrets into a docker config.
func Prepare(ctx context.Context, cfg *config.Config, client kubernetes.Interface) (PreparedCredentials, error) {
	creds, err := LoadFromEnvironment()
	if err != nil {
		return PreparedCredentials{}, err
	}

	if !cfg.UseImagePullSecrets && creds.DockerConfigDir == "" {
		return PreparedCredentials{Credentials: creds}, nil
	}

	configs := make([]dockerConfig, 0)
	if cfg.UseImagePullSecrets {
		if client == nil {
			return PreparedCredentials{}, fmt.Errorf("kubernetes client is required when IMAGE_TRUST_USE_IMAGE_PULL_SECRETS is enabled")
		}
		pullConfigs, err := CollectPullSecretConfigs(ctx, client, cfg.NamespaceAllowlist, cfg.NamespaceBlocklist)
		if err != nil {
			return PreparedCredentials{}, err
		}
		configs = append(configs, pullConfigs...)
		logrus.Infof("loaded docker config from %d imagePullSecret sources", len(pullConfigs))
	}

	if creds.DockerConfigDir != "" {
		envCfg, err := readDockerConfigDir(creds.DockerConfigDir)
		if err != nil {
			return PreparedCredentials{}, fmt.Errorf("reading registry docker config: %w", err)
		}
		configs = append(configs, envCfg)
	}

	merged := mergeDockerConfigs(configs...)
	if creds.Username != "" || creds.Password != "" {
		host := strings.TrimSpace(cfg.RegistryAuthHost)
		if host == "" {
			host = defaultRegistryAuthHost
		}
		merged = merged.withBasicAuth(host, creds.Username, creds.Password)
	}

	tempDir, err := os.MkdirTemp("", "image-trust-docker-config-*")
	if err != nil {
		return PreparedCredentials{}, fmt.Errorf("creating docker config temp dir: %w", err)
	}
	if err := writeDockerConfigDir(tempDir, merged); err != nil {
		_ = os.RemoveAll(tempDir)
		return PreparedCredentials{}, err
	}

	creds.DockerConfigDir = tempDir
	creds.Username = ""
	creds.Password = ""
	applyDockerConfigEnv(creds)
	return PreparedCredentials{
		Credentials: creds,
		cleanup: func() {
			_ = os.RemoveAll(tempDir)
		},
	}, nil
}

// ResolveDockerConfigPath returns the config.json path for a prepared docker config directory.
func ResolveDockerConfigPath(dir string) string {
	return filepath.Join(dir, "config.json")
}

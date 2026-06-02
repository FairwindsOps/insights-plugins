package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultRegistryAuthHost = "https://index.docker.io/v1/"

// PreparedCredentials holds registry settings and temp directories to remove on cleanup.
type PreparedCredentials struct {
	Credentials Credentials
	cleanups    []func()
}

// Cleanup removes temporary directories created during preparation.
func (p PreparedCredentials) Cleanup() {
	for i := len(p.cleanups) - 1; i >= 0; i-- {
		p.cleanups[i]()
	}
}

// Prepare merges injected registry credentials into docker config and prepares TLS bundles.
func Prepare(_ context.Context, creds Credentials) (PreparedCredentials, error) {
	prepared := PreparedCredentials{Credentials: creds}

	if err := prepared.materializeDockerConfig(); err != nil {
		prepared.Cleanup()
		return PreparedCredentials{}, err
	}
	if err := prepared.materializeCertDir(); err != nil {
		prepared.Cleanup()
		return PreparedCredentials{}, err
	}

	applyDockerConfigEnv(prepared.Credentials)
	return prepared, nil
}

func (p *PreparedCredentials) materializeDockerConfig() error {
	creds := &p.Credentials
	needsConfig := creds.DockerConfigDir != "" ||
		creds.Username != "" ||
		creds.Password != "" ||
		len(creds.Auths) > 0

	if !needsConfig {
		return nil
	}

	configs := make([]dockerConfig, 0)

	if creds.DockerConfigDir != "" {
		envCfg, err := readDockerConfigDir(creds.DockerConfigDir)
		if err != nil {
			return fmt.Errorf("reading registry docker config: %w", err)
		}
		configs = append(configs, envCfg)
	}

	for _, auth := range creds.Auths {
		host := normalizeAuthHost(auth.Host)
		configs = append(configs, dockerConfig{}.withBasicAuth(host, auth.Username, auth.Password))
	}

	merged := mergeDockerConfigs(configs...)
	if creds.Username != "" || creds.Password != "" {
		host := strings.TrimSpace(creds.AuthHost)
		if host == "" {
			host = defaultRegistryAuthHost
		}
		merged = merged.withBasicAuth(normalizeAuthHost(host), creds.Username, creds.Password)
	}

	tempDir, err := os.MkdirTemp("", "image-trust-docker-config-*")
	if err != nil {
		return fmt.Errorf("creating docker config temp dir: %w", err)
	}
	if err := writeDockerConfigDir(tempDir, merged); err != nil {
		_ = os.RemoveAll(tempDir)
		return err
	}

	creds.DockerConfigDir = tempDir
	creds.Username = ""
	creds.Password = ""
	p.cleanups = append(p.cleanups, func() { _ = os.RemoveAll(tempDir) })
	return nil
}

func (p *PreparedCredentials) materializeCertDir() error {
	creds := &p.Credentials
	if creds.CertDir != "" && len(creds.PerRegistryCertDirs) == 0 {
		return nil
	}
	if creds.CertDir == "" && len(creds.PerRegistryCertDirs) == 0 {
		return nil
	}

	tempDir, err := os.MkdirTemp("", "image-trust-registry-certs-*")
	if err != nil {
		return err
	}
	if err := MergeRegistryCertDirs(tempDir, creds.CertDir, creds.PerRegistryCertDirs); err != nil {
		_ = os.RemoveAll(tempDir)
		return err
	}
	creds.CertDir = tempDir
	p.cleanups = append(p.cleanups, func() { _ = os.RemoveAll(tempDir) })
	return nil
}

func normalizeAuthHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return host
	}
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "https://" + host
	}
	if !strings.HasSuffix(host, "/v1") && !strings.HasSuffix(host, "/v1/") {
		if strings.HasSuffix(host, "/") {
			host += "v1/"
		} else {
			host += "/v1/"
		}
	}
	return host
}

// ResolveDockerConfigPath returns the config.json path for a prepared docker config directory.
func ResolveDockerConfigPath(dir string) string {
	return filepath.Join(dir, "config.json")
}

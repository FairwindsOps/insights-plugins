package registry

import (
	"fmt"
	"os"
	"strings"
)

// Credentials holds registry authentication settings for cosign.
type Credentials struct {
	Username        string
	Password        string
	CertDir         string
	DockerConfigDir string
}

// LoadFromEnvironment reads registry credentials from the same env vars as the Trivy plugin.
func LoadFromEnvironment() (Credentials, error) {
	creds := Credentials{
		Username: os.Getenv("REGISTRY_USER"),
		Password: os.Getenv("REGISTRY_PASSWORD"),
		CertDir:  os.Getenv("REGISTRY_CERT_DIR"),
	}

	passwordFile := os.Getenv("REGISTRY_PASSWORD_FILE")
	if passwordFile != "" {
		content, err := os.ReadFile(passwordFile)
		if err != nil {
			return Credentials{}, fmt.Errorf("reading registry password file: %w", err)
		}
		creds.Password = strings.TrimSpace(string(content))
	}

	dockerConfigPath := strings.TrimSpace(os.Getenv("REGISTRY_DOCKER_CONFIG_PATH"))
	if dockerConfigPath != "" {
		dir, err := resolveDockerConfigDir(dockerConfigPath)
		if err != nil {
			return Credentials{}, err
		}
		creds.DockerConfigDir = dir
	}

	applyDockerConfigEnv(creds)
	return creds, nil
}

func applyDockerConfigEnv(creds Credentials) {
	if creds.DockerConfigDir != "" {
		_ = os.Setenv("DOCKER_CONFIG", creds.DockerConfigDir)
	}
	if creds.CertDir != "" {
		_ = os.Setenv("SSL_CERT_DIR", creds.CertDir)
	}
}

// Configured reports whether registry credentials or custom TLS settings are set.
func (c Credentials) Configured() bool {
	return c.Username != "" || c.Password != "" || c.CertDir != "" || c.DockerConfigDir != ""
}

// CosignArgs returns cosign CLI flags for registry authentication.
// When DockerConfigDir is set, per-registry auth comes from config.json instead.
func (c Credentials) CosignArgs() []string {
	if c.DockerConfigDir != "" {
		return nil
	}
	var args []string
	if c.Username != "" {
		args = append(args, "--registry-username", c.Username)
	}
	if c.Password != "" {
		args = append(args, "--registry-password", c.Password)
	}
	return args
}

// ExtraEnv returns environment variables to pass to cosign subprocesses.
func (c Credentials) ExtraEnv() []string {
	var env []string
	if c.CertDir != "" {
		env = append(env, "SSL_CERT_DIR="+c.CertDir)
	}
	if c.DockerConfigDir != "" {
		env = append(env, "DOCKER_CONFIG="+c.DockerConfigDir)
	}
	return env
}

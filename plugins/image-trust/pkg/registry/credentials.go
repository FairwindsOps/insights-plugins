package registry

import (
	"fmt"
	"os"
	"strings"
)

// Credentials holds registry authentication and TLS settings for registry API and cosign.
type Credentials struct {
	Username            string
	Password            string
	CertDir             string
	DockerConfigDir     string
	PerRegistryCertDirs map[string]string
	Mirrors             map[string]string
}

// LoadFromEnvironment reads registry credentials from environment variables.
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
	return c.Username != "" || c.Password != "" || c.CertDir != "" || c.DockerConfigDir != "" || len(c.PerRegistryCertDirs) > 0
}

// CosignArgs returns cosign CLI flags for registry authentication.
// Credentials are always materialized into docker config.json; passwords are never passed on the CLI.
func (c Credentials) CosignArgs() []string {
	return nil
}

// ExtraEnv returns environment variables to pass to cosign subprocesses.
func (c Credentials) ExtraEnv(extra ...string) []string {
	env := append([]string(nil), extra...)
	if c.CertDir != "" {
		env = append(env, "SSL_CERT_DIR="+c.CertDir)
	}
	if c.DockerConfigDir != "" {
		env = append(env, "DOCKER_CONFIG="+c.DockerConfigDir)
	}
	return env
}

// VerificationReference applies mirror remapping to an immutable reference.
func (c Credentials) VerificationReference(ref string) string {
	return RemapMirror(ref, c.Mirrors)
}

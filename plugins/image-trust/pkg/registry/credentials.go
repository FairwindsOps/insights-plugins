package registry

import (
	"os"
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

package registry

import (
	"os"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/config"
)

// Credentials holds registry authentication and TLS settings for registry API and cosign.
type Credentials struct {
	Username            string
	Password            string
	CertDir             string
	DockerConfigDir     string
	AuthHost            string
	Auths               []config.RegistryAuth
	PerRegistryCertDirs map[string]string
	Mirrors             map[string]string
}

// CredentialsFromConfig builds registry credentials from loaded plugin configuration.
// Registry settings must be populated by config.LoadFromEnvironment before calling this.
func CredentialsFromConfig(cfg *config.Config) Credentials {
	if cfg == nil {
		return Credentials{
			Auths:               []config.RegistryAuth{},
			Mirrors:             map[string]string{},
			PerRegistryCertDirs: map[string]string{},
		}
	}
	auths := cfg.RegistryAuths
	if auths == nil {
		auths = []config.RegistryAuth{}
	}
	mirrors := cfg.RegistryMirrors
	if mirrors == nil {
		mirrors = map[string]string{}
	}
	certDirs := cfg.RegistryCertDirs
	if certDirs == nil {
		certDirs = map[string]string{}
	}
	return Credentials{
		Username:            cfg.RegistryUser,
		Password:            cfg.RegistryPassword,
		CertDir:             cfg.RegistryCertDir,
		DockerConfigDir:     cfg.RegistryDockerConfigDir,
		AuthHost:            cfg.RegistryAuthHost,
		Auths:               auths,
		Mirrors:             mirrors,
		PerRegistryCertDirs: certDirs,
	}
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
	return c.Username != "" || c.Password != "" || c.CertDir != "" || c.DockerConfigDir != "" || len(c.PerRegistryCertDirs) > 0 || len(c.Auths) > 0
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

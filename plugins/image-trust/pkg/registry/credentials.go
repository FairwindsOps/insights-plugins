package registry

import (
	"fmt"
	"os"
	"strings"
)

// Credentials holds registry authentication settings for cosign.
type Credentials struct {
	Username string
	Password string
	CertDir  string
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

	return creds, nil
}

// Configured reports whether registry credentials or custom TLS settings are set.
func (c Credentials) Configured() bool {
	return c.Username != "" || c.Password != "" || c.CertDir != ""
}

// CosignArgs returns cosign CLI flags for registry authentication.
func (c Credentials) CosignArgs() []string {
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
	if c.CertDir == "" {
		return nil
	}
	return []string{"SSL_CERT_DIR=" + c.CertDir}
}

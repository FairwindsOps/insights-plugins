package sigstore

import (
	"bufio"
	"os"
	"strings"
)

// Well-known Sigstore/Cosign environment variables for private or air-gapped deployments.
var wellKnownEnvVars = []string{
	"SIGSTORE_ROOT_FILE",
	"COSIGN_ROOT",
	"SIGSTORE_CT_LOG_PUBLIC_KEY_FILE",
	"FULCIO_URL",
	"REKOR_URL",
	"TUF_ROOT",
	"SIGSTORE_FULCIO_URL",
	"SIGSTORE_REKOR_URL",
	"SIGSTORE_CT_LOG_PUBLIC_KEY_FILE",
	"SIGSTORE_TUF_ROOT",
	"COSIGN_EXPERIMENTAL",
	"GOOGLE_APPLICATION_CREDENTIALS",
	"AWS_REGION",
	"AWS_DEFAULT_REGION",
	"AWS_ROLE_ARN",
	"AWS_WEB_IDENTITY_TOKEN_FILE",
	"AZURE_CLIENT_ID",
	"AZURE_TENANT_ID",
	"AZURE_FEDERATED_TOKEN_FILE",
}

// ExtraEnv returns Sigstore-related variables to forward to cosign subprocesses.
// Set IMAGE_TRUST_SIGSTORE_ENV_FILE for additional KEY=VALUE lines (one per line).
func ExtraEnv() ([]string, error) {
	seen := map[string]struct{}{}
	var env []string

	appendVar := func(key, value string) {
		if value == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		env = append(env, key+"="+value)
	}

	for _, key := range wellKnownEnvVars {
		appendVar(key, os.Getenv(key))
	}

	file := strings.TrimSpace(os.Getenv("IMAGE_TRUST_SIGSTORE_ENV_FILE"))
	if file == "" {
		return env, nil
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		appendVar(strings.TrimSpace(key), strings.TrimSpace(value))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return env, nil
}

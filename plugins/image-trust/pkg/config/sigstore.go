package config

import (
	"os"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/sigstore"
)

func loadSigstoreSettings() (envFile string, env []string, err error) {
	envFile = strings.TrimSpace(os.Getenv("IMAGE_TRUST_SIGSTORE_ENV_FILE"))

	vars := make(map[string]string)
	for _, key := range sigstore.WellKnownEnvVarKeys() {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			vars[key] = value
		}
	}

	env, err = sigstore.LoadEnv(sigstore.EnvInput{
		EnvFile: envFile,
		Vars:    vars,
	})
	return envFile, env, err
}

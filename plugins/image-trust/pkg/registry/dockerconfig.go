package registry

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type dockerConfig struct {
	Auths       map[string]dockerConfigAuth `json:"auths,omitempty"`
	CredHelpers map[string]string           `json:"credHelpers,omitempty"`
}

type dockerConfigAuth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Auth     string `json:"auth,omitempty"`
	Identity string `json:"identitytoken,omitempty"`
}

func parseDockerConfig(data []byte) (dockerConfig, error) {
	var cfg dockerConfig
	if len(data) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return dockerConfig{}, fmt.Errorf("parsing docker config: %w", err)
	}
	if cfg.Auths == nil {
		cfg.Auths = map[string]dockerConfigAuth{}
	}
	if cfg.CredHelpers == nil {
		cfg.CredHelpers = map[string]string{}
	}
	return cfg, nil
}

func readDockerConfigFile(path string) (dockerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return dockerConfig{}, err
	}
	return parseDockerConfig(data)
}

func readDockerConfigDir(dir string) (dockerConfig, error) {
	return readDockerConfigFile(strings.TrimRight(dir, "/") + "/config.json")
}

// mergeDockerConfigs overlays configs left-to-right; later entries override the same host keys.
func mergeDockerConfigs(configs ...dockerConfig) dockerConfig {
	merged := dockerConfig{
		Auths:       map[string]dockerConfigAuth{},
		CredHelpers: map[string]string{},
	}
	for _, cfg := range configs {
		for host, auth := range cfg.Auths {
			merged.Auths[host] = auth
		}
		for host, helper := range cfg.CredHelpers {
			merged.CredHelpers[host] = helper
		}
	}
	return merged
}

func (c dockerConfig) withBasicAuth(host, username, password string) dockerConfig {
	if username == "" && password == "" {
		return c
	}
	if c.Auths == nil {
		c.Auths = map[string]dockerConfigAuth{}
	}
	c.Auths[host] = dockerConfigAuth{
		Username: username,
		Password: password,
		Auth:     base64.StdEncoding.EncodeToString([]byte(username + ":" + password)),
	}
	return c
}

func writeDockerConfigDir(dir string, cfg dockerConfig) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(strings.TrimRight(dir, "/")+"/config.json", data, 0o600)
}

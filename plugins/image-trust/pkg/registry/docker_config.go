package registry

import (
	"fmt"
	"os"
	"path/filepath"
)

// resolveDockerConfigDir returns the directory cosign should use via DOCKER_CONFIG.
// path may be a directory containing config.json or a direct path to config.json.
func resolveDockerConfigDir(path string) (string, error) {
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("registry docker config %s: %w", path, err)
	}

	if info.IsDir() {
		configPath := filepath.Join(path, "config.json")
		if _, err := os.Stat(configPath); err != nil {
			return "", fmt.Errorf("registry docker config directory %s missing config.json: %w", path, err)
		}
		return path, nil
	}

	if filepath.Base(path) != "config.json" {
		return "", fmt.Errorf("registry docker config file %s must be named config.json", path)
	}
	return filepath.Dir(path), nil
}

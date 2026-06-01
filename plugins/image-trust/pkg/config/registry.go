package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RegistryAuth is credentials for one registry host key in docker config auths.
type RegistryAuth struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// registrySettings holds registry auth, mirrors, TLS, and docker config loaded at startup.
type registrySettings struct {
	Auths               []RegistryAuth
	Mirrors             map[string]string
	CertDirs            map[string]string
	User                string
	Password            string
	CertDir             string
	DockerConfigDir     string
}

func loadRegistrySettings() (registrySettings, error) {
	auths, err := loadRegistryAuths()
	if err != nil {
		return registrySettings{}, err
	}
	mirrors, err := loadRegistryMirrors()
	if err != nil {
		return registrySettings{}, err
	}
	certDirs, err := loadRegistryCertDirs()
	if err != nil {
		return registrySettings{}, err
	}
	user, password, certDir, dockerConfigDir, err := loadRegistryCredentials()
	if err != nil {
		return registrySettings{}, err
	}
	return registrySettings{
		Auths:           auths,
		Mirrors:         mirrors,
		CertDirs:        certDirs,
		User:            user,
		Password:        password,
		CertDir:         certDir,
		DockerConfigDir: dockerConfigDir,
	}, nil
}

func loadRegistryAuths() ([]RegistryAuth, error) {
	raw, err := envValueOrFile("IMAGE_TRUST_REGISTRY_AUTHS", "IMAGE_TRUST_REGISTRY_AUTHS_FILE")
	if err != nil {
		return nil, err
	}
	return parseRegistryAuths(raw)
}

func parseRegistryAuths(raw string) ([]RegistryAuth, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var auths []RegistryAuth
	if err := json.Unmarshal([]byte(raw), &auths); err != nil {
		return nil, fmt.Errorf("parsing IMAGE_TRUST_REGISTRY_AUTHS: %w", err)
	}
	for i, auth := range auths {
		if strings.TrimSpace(auth.Host) == "" {
			return nil, fmt.Errorf("registry auth entry %d missing host", i)
		}
	}
	return auths, nil
}

func loadRegistryMirrors() (map[string]string, error) {
	raw, err := envValueOrFile("IMAGE_TRUST_REGISTRY_MIRRORS", "IMAGE_TRUST_REGISTRY_MIRRORS_FILE")
	if err != nil {
		return nil, err
	}
	return parseRegistryMirrors(raw)
}

func parseRegistryMirrors(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	mirrors := map[string]string{}
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid registry mirror pair %q (expected mirror=upstream)", pair)
		}
		mirror := strings.TrimSpace(parts[0])
		upstream := strings.TrimSpace(parts[1])
		if mirror == "" || upstream == "" {
			return nil, fmt.Errorf("invalid registry mirror pair %q", pair)
		}
		mirrors[mirror] = upstream
	}
	return mirrors, nil
}

func loadRegistryCertDirs() (map[string]string, error) {
	raw, err := envValueOrFile("IMAGE_TRUST_REGISTRY_CERT_DIRS", "IMAGE_TRUST_REGISTRY_CERT_DIRS_FILE")
	if err != nil {
		return nil, err
	}
	return parseRegistryCertDirs(raw)
}

func parseRegistryCertDirs(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	dirs := map[string]string{}
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid registry cert dir pair %q (expected host=/path)", pair)
		}
		host := strings.TrimSpace(parts[0])
		dir := strings.TrimSpace(parts[1])
		if host == "" || dir == "" {
			return nil, fmt.Errorf("invalid registry cert dir pair %q", pair)
		}
		dirs[host] = dir
	}
	return dirs, nil
}

func loadRegistryCredentials() (username, password, certDir, dockerConfigDir string, err error) {
	username = strings.TrimSpace(os.Getenv("REGISTRY_USER"))
	password = strings.TrimSpace(os.Getenv("REGISTRY_PASSWORD"))
	certDir = strings.TrimSpace(os.Getenv("REGISTRY_CERT_DIR"))

	passwordFile := strings.TrimSpace(os.Getenv("REGISTRY_PASSWORD_FILE"))
	if passwordFile != "" {
		content, readErr := os.ReadFile(passwordFile)
		if readErr != nil {
			return "", "", "", "", fmt.Errorf("reading REGISTRY_PASSWORD_FILE: %w", readErr)
		}
		password = strings.TrimSpace(string(content))
	}

	dockerConfigPath := strings.TrimSpace(os.Getenv("REGISTRY_DOCKER_CONFIG_PATH"))
	if dockerConfigPath != "" {
		dockerConfigDir, err = resolveDockerConfigDir(dockerConfigPath)
		if err != nil {
			return "", "", "", "", err
		}
	}

	return username, password, certDir, dockerConfigDir, nil
}

func envValueOrFile(envKey, fileEnvKey string) (string, error) {
	raw := strings.TrimSpace(os.Getenv(envKey))
	if path := strings.TrimSpace(os.Getenv(fileEnvKey)); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("reading %s: %w", fileEnvKey, err)
		}
		raw = string(data)
	}
	return raw, nil
}

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

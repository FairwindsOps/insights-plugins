package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// RegistryAuth is credentials for one registry host key in docker config auths.
type RegistryAuth struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoadRegistryAuths reads IMAGE_TRUST_REGISTRY_AUTHS (JSON array) or _FILE.
func LoadRegistryAuths() ([]RegistryAuth, error) {
	raw := strings.TrimSpace(os.Getenv("IMAGE_TRUST_REGISTRY_AUTHS"))
	if path := strings.TrimSpace(os.Getenv("IMAGE_TRUST_REGISTRY_AUTHS_FILE")); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading IMAGE_TRUST_REGISTRY_AUTHS_FILE: %w", err)
		}
		raw = string(data)
	}
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

// LoadRegistryMirrors reads IMAGE_TRUST_REGISTRY_MIRRORS as comma-separated mirror=upstream pairs.
func LoadRegistryMirrors() (map[string]string, error) {
	raw := strings.TrimSpace(os.Getenv("IMAGE_TRUST_REGISTRY_MIRRORS"))
	if path := strings.TrimSpace(os.Getenv("IMAGE_TRUST_REGISTRY_MIRRORS_FILE")); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading IMAGE_TRUST_REGISTRY_MIRRORS_FILE: %w", err)
		}
		raw = string(data)
	}
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

// LoadRegistryCertDirs reads IMAGE_TRUST_REGISTRY_CERT_DIRS as comma-separated host=/path pairs.
func LoadRegistryCertDirs() (map[string]string, error) {
	raw := strings.TrimSpace(os.Getenv("IMAGE_TRUST_REGISTRY_CERT_DIRS"))
	if path := strings.TrimSpace(os.Getenv("IMAGE_TRUST_REGISTRY_CERT_DIRS_FILE")); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading IMAGE_TRUST_REGISTRY_CERT_DIRS_FILE: %w", err)
		}
		raw = string(data)
	}
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

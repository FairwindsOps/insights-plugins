package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	MaxPublicKeyFiles = 32
	MaxPublicKeyBytes = 64 * 1024
)

// TrustedPublicKey is a public key file used for cosign-key verification.
type TrustedPublicKey struct {
	Path string
	ID   string
}

// LoadTrustedPublicKeys resolves configured public key paths for cosign-key mode.
func LoadTrustedPublicKeys(paths []string, dir string) ([]TrustedPublicKey, error) {
	resolved := make([]string, 0, len(paths))
	seen := make(map[string]struct{})

	for _, raw := range paths {
		path := strings.TrimSpace(raw)
		if path == "" {
			continue
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolving public key path %q: %w", path, err)
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		resolved = append(resolved, abs)
	}

	if strings.TrimSpace(dir) != "" {
		dirAbs, err := filepath.Abs(strings.TrimSpace(dir))
		if err != nil {
			return nil, fmt.Errorf("resolving public key directory: %w", err)
		}
		entries, err := os.ReadDir(dirAbs)
		if err != nil {
			return nil, fmt.Errorf("reading public key directory %s: %w", dirAbs, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			lower := strings.ToLower(name)
			if !strings.HasSuffix(lower, ".pub") && !strings.HasSuffix(lower, ".pem") {
				continue
			}
			abs := filepath.Join(dirAbs, name)
			if _, ok := seen[abs]; ok {
				continue
			}
			seen[abs] = struct{}{}
			resolved = append(resolved, abs)
		}
	}

	if len(resolved) == 0 {
		return nil, fmt.Errorf("no trusted public keys configured")
	}
	if len(resolved) > MaxPublicKeyFiles {
		return nil, fmt.Errorf("at most %d trusted public keys are supported", MaxPublicKeyFiles)
	}

	keys := make([]TrustedPublicKey, 0, len(resolved))
	for _, path := range resolved {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("public key %s: %w", path, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("public key %s is a directory", path)
		}
		if info.Size() > MaxPublicKeyBytes {
			return nil, fmt.Errorf("public key %s exceeds maximum size of %d bytes", path, MaxPublicKeyBytes)
		}
		keys = append(keys, TrustedPublicKey{
			Path: path,
			ID:   filepath.Base(path),
		})
	}
	return keys, nil
}

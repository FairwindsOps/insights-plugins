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

// TrustedPublicKey is a public key reference used for cosign-key verification.
// Ref is passed to cosign --key (local file path or remote URI such as gcpkms://).
type TrustedPublicKey struct {
	Ref string
	ID  string
}

// ReportKeyRef returns the key identifier to include in verification reports.
// Remote refs (URLs, KMS URIs) are reported verbatim; local files use the basename.
func (k TrustedPublicKey) ReportKeyRef() string {
	if isRemoteKeyRef(k.Ref) {
		return k.Ref
	}
	return k.ID
}

func isRemoteKeyRef(ref string) bool {
	return strings.Contains(ref, "://")
}

// LoadTrustedPublicKeys resolves configured public key paths and remote refs for cosign-key mode.
func LoadTrustedPublicKeys(paths, refs []string, dir string) ([]TrustedPublicKey, error) {
	resolved := make([]string, 0, len(paths)+len(refs))
	seen := make(map[string]struct{})

	addRef := func(ref string) error {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return nil
		}
		if !isRemoteKeyRef(ref) {
			abs, err := filepath.Abs(ref)
			if err != nil {
				return fmt.Errorf("resolving public key path %q: %w", ref, err)
			}
			ref = abs
		}
		if _, ok := seen[ref]; ok {
			return nil
		}
		if len(resolved) >= MaxPublicKeyFiles {
			return fmt.Errorf("at most %d trusted public keys are supported", MaxPublicKeyFiles)
		}
		seen[ref] = struct{}{}
		resolved = append(resolved, ref)
		return nil
	}

	for _, raw := range refs {
		ref := strings.TrimSpace(raw)
		if ref == "" {
			continue
		}
		if !isRemoteKeyRef(ref) {
			return nil, fmt.Errorf("public key ref %q must be a URI (for example gcpkms://...)", ref)
		}
		if err := addRef(ref); err != nil {
			return nil, err
		}
	}

	for _, raw := range paths {
		if err := addRef(raw); err != nil {
			return nil, err
		}
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
			if err := addRef(filepath.Join(dirAbs, name)); err != nil {
				return nil, err
			}
		}
	}

	if len(resolved) == 0 {
		return nil, fmt.Errorf("no trusted public keys configured")
	}

	keys := make([]TrustedPublicKey, 0, len(resolved))
	for _, ref := range resolved {
		if !isRemoteKeyRef(ref) {
			info, err := os.Stat(ref)
			if err != nil {
				return nil, fmt.Errorf("public key %s: %w", ref, err)
			}
			if info.IsDir() {
				return nil, fmt.Errorf("public key %s is a directory", ref)
			}
			if info.Size() > MaxPublicKeyBytes {
				return nil, fmt.Errorf("public key %s exceeds maximum size of %d bytes", ref, MaxPublicKeyBytes)
			}
		}
		keys = append(keys, TrustedPublicKey{
			Ref: ref,
			ID:  keyID(ref),
		})
	}
	return keys, nil
}

func keyID(ref string) string {
	if isRemoteKeyRef(ref) {
		if idx := strings.LastIndex(ref, "/"); idx >= 0 {
			return ref[idx+1:]
		}
		return ref
	}
	return filepath.Base(ref)
}

package registry

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// MergeRegistryCertDirs copies PEM/CRT files from per-registry cert dirs into targetDir for SSL_CERT_DIR.
func MergeRegistryCertDirs(targetDir string, globalCertDir string, perRegistry map[string]string) error {
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return err
	}

	dirs := make([]string, 0, len(perRegistry)+1)
	if globalCertDir != "" {
		dirs = append(dirs, globalCertDir)
	}
	seen := map[string]struct{}{}
	for _, dir := range perRegistry {
		if dir == "" {
			continue
		}
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		dirs = append(dirs, dir)
	}

	index := 0
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("reading cert dir %s: %w", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := strings.ToLower(entry.Name())
			if !strings.HasSuffix(name, ".pem") && !strings.HasSuffix(name, ".crt") && !strings.HasSuffix(name, ".cer") {
				continue
			}
			src := filepath.Join(dir, entry.Name())
			dst := filepath.Join(targetDir, fmt.Sprintf("bundle-%d-%s", index, entry.Name()))
			data, err := os.ReadFile(src)
			if err != nil {
				return fmt.Errorf("reading cert %s: %w", src, err)
			}
			if err := os.WriteFile(dst, data, 0o600); err != nil {
				return err
			}
			index++
		}
	}
	return nil
}

// TransportForReference returns an http.Transport with custom roots when per-registry TLS is configured.
func TransportForReference(ref string, globalCertDir string, perRegistry map[string]string) (*http.Transport, error) {
	certDir := certDirForReference(ref, globalCertDir, perRegistry)
	if certDir == "" && len(perRegistry) == 0 && globalCertDir == "" {
		return http.DefaultTransport.(*http.Transport).Clone(), nil
	}

	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if certDir != "" {
		if err := appendCertsFromDir(pool, certDir); err != nil {
			return nil, err
		}
	}
	for _, dir := range perRegistry {
		if err := appendCertsFromDir(pool, dir); err != nil {
			return nil, err
		}
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		RootCAs:    pool,
		MinVersion: tls.VersionTLS12,
	}
	return transport, nil
}

func certDirForReference(ref string, globalCertDir string, perRegistry map[string]string) string {
	host := registryHostFromReference(ref)
	if host == "" {
		return globalCertDir
	}
	if dir, ok := perRegistry[host]; ok {
		return dir
	}
	for key, dir := range perRegistry {
		if strings.EqualFold(key, host) {
			return dir
		}
	}
	return globalCertDir
}

func registryHostFromReference(ref string) string {
	candidate := ref
	if idx := strings.Index(candidate, "@"); idx >= 0 {
		candidate = candidate[:idx]
	}
	if slash := strings.Index(candidate, "/"); slash >= 0 {
		candidate = candidate[:slash]
	}
	if colon := strings.LastIndex(candidate, ":"); colon >= 0 {
		candidate = candidate[:colon]
	}
	return candidate
}

func appendCertsFromDir(pool *x509.CertPool, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if !strings.HasSuffix(name, ".pem") && !strings.HasSuffix(name, ".crt") && !strings.HasSuffix(name, ".cer") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return err
		}
		if !pool.AppendCertsFromPEM(data) {
			return fmt.Errorf("no certificates parsed from %s/%s", dir, entry.Name())
		}
	}
	return nil
}

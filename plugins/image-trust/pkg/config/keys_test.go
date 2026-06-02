package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadTrustedPublicKeysFromPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "release.pub")
	require.NoError(t, os.WriteFile(path, []byte("-----BEGIN PUBLIC KEY-----\nabc\n-----END PUBLIC KEY-----\n"), 0o644))

	keys, err := LoadTrustedPublicKeys([]string{path}, nil, "")
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, path, keys[0].Ref)
	require.Equal(t, "release.pub", keys[0].ID)
}

func TestLoadTrustedPublicKeysFromDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.pub"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("b"), 0o644))

	keys, err := LoadTrustedPublicKeys(nil, nil, dir)
	require.NoError(t, err)
	require.Len(t, keys, 1)
}

func TestLoadTrustedPublicKeysFromRemoteRef(t *testing.T) {
	keys, err := LoadTrustedPublicKeys(nil, []string{"gcpkms://projects/p/locations/l/keyRings/r/cryptoKeys/k/versions/1"}, "")
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, "1", keys[0].ID)
	require.Equal(t, "gcpkms://projects/p/locations/l/keyRings/r/cryptoKeys/k/versions/1", keys[0].ReportKeyRef())
}

func TestTrustedPublicKeyReportKeyRefHTTPS(t *testing.T) {
	key := TrustedPublicKey{
		Ref: "https://artifacts.fairwinds.com/cosign-p256.pub",
		ID:  "cosign-p256.pub",
	}
	require.Equal(t, key.Ref, key.ReportKeyRef())
}

func TestTrustedPublicKeyReportKeyRefLocalFile(t *testing.T) {
	key := TrustedPublicKey{
		Ref: "/etc/image-trust/keys/release.pub",
		ID:  "release.pub",
	}
	require.Equal(t, "release.pub", key.ReportKeyRef())
}

func TestLoadTrustedPublicKeysRequiresInput(t *testing.T) {
	_, err := LoadTrustedPublicKeys(nil, nil, "")
	require.Error(t, err)
}

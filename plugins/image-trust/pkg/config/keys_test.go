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

	keys, err := LoadTrustedPublicKeys([]string{path}, "")
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, path, keys[0].Path)
	require.Equal(t, "release.pub", keys[0].ID)
}

func TestLoadTrustedPublicKeysFromDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.pub"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("b"), 0o644))

	keys, err := LoadTrustedPublicKeys(nil, dir)
	require.NoError(t, err)
	require.Len(t, keys, 1)
}

func TestLoadTrustedPublicKeysRequiresInput(t *testing.T) {
	_, err := LoadTrustedPublicKeys(nil, "")
	require.Error(t, err)
}

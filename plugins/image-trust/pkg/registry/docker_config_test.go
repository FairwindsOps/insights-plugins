package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveDockerConfigDirFromDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{}`), 0o600))

	resolved, err := resolveDockerConfigDir(dir)
	require.NoError(t, err)
	require.Equal(t, dir, resolved)
}

func TestResolveDockerConfigDirFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{}`), 0o600))

	resolved, err := resolveDockerConfigDir(path)
	require.NoError(t, err)
	require.Equal(t, dir, resolved)
}

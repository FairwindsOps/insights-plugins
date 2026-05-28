package registry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemapMirror(t *testing.T) {
	ref := RemapMirror("mirror.corp/project/image@sha256:abc", map[string]string{
		"mirror.corp": "registry.io",
	})
	require.Equal(t, "registry.io/project/image@sha256:abc", ref)
}

func TestRemapMirrorNoMatch(t *testing.T) {
	ref := RemapMirror("ghcr.io/org/image:1", map[string]string{"mirror.corp": "registry.io"})
	require.Equal(t, "ghcr.io/org/image:1", ref)
}

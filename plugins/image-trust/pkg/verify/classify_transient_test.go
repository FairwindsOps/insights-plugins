package verify

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsTransientFailure(t *testing.T) {
	require.True(t, IsTransientFailure("Get \"https://rekor.sigstore.dev\": dial tcp i/o timeout"))
	require.False(t, IsTransientFailure("no matching signatures"))
}

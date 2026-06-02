package verify

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildAlternationRegex(t *testing.T) {
	pattern, err := buildAlternationRegex(
		[]string{"https://issuer.example"},
		[]string{"https://github.com/example/.+"},
	)
	require.NoError(t, err)
	require.Contains(t, pattern, "^https://issuer\\.example$")
	require.Contains(t, pattern, "https://github.com/example/.+")
}

func TestExtractCosignSigners(t *testing.T) {
	signers, err := extractCosignSigners(`[{"optional":{"Issuer":"https://token.actions.githubusercontent.com","Subject":"https://github.com/example/repo/.github/workflows/build.yml@refs/heads/main","keyid":"abc123"}}]`)
	require.NoError(t, err)
	require.Len(t, signers, 1)
	require.Equal(t, "https://token.actions.githubusercontent.com", signers[0].Issuer)
	require.Equal(t, "https://github.com/example/repo/.github/workflows/build.yml@refs/heads/main", signers[0].Subject)
	require.Equal(t, "abc123", signers[0].KeyRef)
}

func TestExtractCosignSignersMultipleRecords(t *testing.T) {
	signers, err := extractCosignSigners(`[
		{"optional":{"Issuer":"https://accounts.google.com","Subject":"other@example.com"}},
		{"optional":{"Issuer":"https://token.actions.githubusercontent.com","Subject":"https://github.com/example/repo/.github/workflows/build.yml@refs/heads/main"}}
	]`)
	require.NoError(t, err)
	require.Len(t, signers, 2)
	require.Equal(t, "other@example.com", signers[0].Subject)
	require.Contains(t, signers[1].Subject, "github.com/example")
}

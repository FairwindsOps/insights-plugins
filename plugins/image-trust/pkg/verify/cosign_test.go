package verify

import (
	"context"
	"errors"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/stretchr/testify/require"
)

type fakeRunner struct {
	stdout string
	stderr string
	err    error
	name   string
	args   []string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, string, error) {
	f.name = name
	f.args = args
	return f.stdout, f.stderr, f.err
}

func TestCosignVerifierVerifySuccess(t *testing.T) {
	runner := &fakeRunner{stdout: "[]"}
	verifier, err := NewCosignVerifier(
		runner,
		[]string{"https://token.actions.githubusercontent.com"},
		nil,
		[]string{"https://github.com/example/.+"},
	)
	require.NoError(t, err)

	observation, err := verifier.Verify(context.Background(), models.DiscoveredImage{
		Name: "ghcr.io/example/api:1.0.0",
		ID:   "ghcr.io/example/api@sha256:abc",
	})
	require.NoError(t, err)
	require.Equal(t, models.StatusVerified, observation.Status)
	require.Equal(t, "cosign", runner.name)
	require.Contains(t, runner.args, "--certificate-identity-regexp")
	require.Contains(t, runner.args, "--certificate-oidc-issuer-regexp")
	require.Contains(t, runner.args, "ghcr.io/example/api@sha256:abc")
}

func TestCosignVerifierVerifyUnsigned(t *testing.T) {
	runner := &fakeRunner{
		stderr: "Error: no matching signatures found",
		err:    errors.New("exit status 1"),
	}
	verifier, err := NewCosignVerifier(runner, nil, nil, nil)
	require.NoError(t, err)

	observation, err := verifier.Verify(context.Background(), models.DiscoveredImage{
		Name: "ghcr.io/example/api:1.0.0",
		ID:   "ghcr.io/example/api@sha256:abc",
	})
	require.NoError(t, err)
	require.Equal(t, models.StatusUnsigned, observation.Status)
}

func TestCosignVerifierVerifySignedUntrusted(t *testing.T) {
	runner := &fakeRunner{
		stderr: "Error: certificate identity https://github.com/other/repo did not match expected identities",
		err:    errors.New("exit status 1"),
	}
	verifier, err := NewCosignVerifier(runner, nil, nil, nil)
	require.NoError(t, err)

	observation, err := verifier.Verify(context.Background(), models.DiscoveredImage{
		Name: "ghcr.io/example/api:1.0.0",
		ID:   "ghcr.io/example/api@sha256:abc",
	})
	require.NoError(t, err)
	require.Equal(t, models.StatusSignedUntrusted, observation.Status)
}

func TestCosignVerifierVerifyUnknownWhenDigestMissing(t *testing.T) {
	runner := &fakeRunner{}
	verifier, err := NewCosignVerifier(runner, nil, nil, nil)
	require.NoError(t, err)

	observation, err := verifier.Verify(context.Background(), models.DiscoveredImage{
		Name: "ghcr.io/example/api:1.0.0",
	})
	require.NoError(t, err)
	require.Equal(t, models.StatusUnknown, observation.Status)
	require.Empty(t, runner.name)
}

func TestBuildAlternationRegex(t *testing.T) {
	pattern, err := buildAlternationRegex(
		[]string{"https://issuer.example"},
		[]string{"https://github.com/example/.+"},
	)
	require.NoError(t, err)
	require.Contains(t, pattern, "^https://issuer\\.example$")
	require.Contains(t, pattern, "https://github.com/example/.+")
}

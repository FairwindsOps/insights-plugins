package verify

import (
	"context"
	"errors"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
	"github.com/stretchr/testify/require"
)

func TestCosignAttestationVerifierVerifySuccess(t *testing.T) {
	runner := &fakeRunner{stdout: `[{"optional":{"Issuer":"https://token.actions.githubusercontent.com","Subject":"https://github.com/example/repo/.github/workflows/build.yml@refs/heads/main"}}]`}
	verifier, err := NewCosignAttestationVerifier(
		runner,
		registry.Credentials{},
		[]string{"slsaprovenance1"},
		[]string{"https://token.actions.githubusercontent.com"},
		nil,
		[]string{"https://github.com/example/.+"},
	)
	require.NoError(t, err)

	observation, err := verifier.Verify(context.Background(), models.DiscoveredImage{
		ID: "ghcr.io/example/api@sha256:abc",
	})
	require.NoError(t, err)
	require.Equal(t, models.StatusVerified, observation.Status)
	require.Equal(t, "slsaprovenance1", observation.AttestationType)
	require.Contains(t, runner.args, "verify-attestation")
	require.Contains(t, runner.args, "--type")
	require.Contains(t, runner.args, "slsaprovenance1")
}

func TestCosignAttestationVerifierUnsigned(t *testing.T) {
	runner := &fakeRunner{
		stderr: "Error: no matching attestations found",
		err:    errors.New("exit status 1"),
	}
	verifier, err := NewCosignAttestationVerifier(
		runner,
		registry.Credentials{},
		[]string{"slsaprovenance1"},
		[]string{"https://token.actions.githubusercontent.com"},
		nil,
		nil,
	)
	require.NoError(t, err)

	observation, err := verifier.Verify(context.Background(), models.DiscoveredImage{
		ID: "ghcr.io/example/api@sha256:abc",
	})
	require.NoError(t, err)
	require.Equal(t, models.StatusUnsigned, observation.Status)
}

package verify

import (
	"context"
	"errors"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/config"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
	"github.com/stretchr/testify/require"
)

func TestCosignKeyVerifierVerifySuccess(t *testing.T) {
	runner := &fakeRunner{stdout: `[{"optional":{"keyid":"deadbeef"}}]`}
	verifier, err := NewCosignKeyVerifier(
		runner,
		registry.Credentials{Username: "user", Password: "pass"},
		[]config.TrustedPublicKey{{Ref: "/etc/image-trust/keys/release.pub", ID: "release.pub"}},
		true,
	)
	require.NoError(t, err)

	observation, err := verifier.Verify(context.Background(), models.DiscoveredImage{
		Name: "ghcr.io/example/api:1.0.0",
		ID:   "ghcr.io/example/api@sha256:abc",
	})
	require.NoError(t, err)
	require.Equal(t, models.StatusVerified, observation.Status)
	require.Equal(t, "release.pub", observation.Signer.KeyRef)
	require.Equal(t, "cosign", runner.name)
	require.Contains(t, runner.args, "--key")
	require.Contains(t, runner.args, "/etc/image-trust/keys/release.pub")
	require.Contains(t, runner.args, "--insecure-ignore-tlog")
	require.Contains(t, runner.args, "--registry-username")
}

func TestCosignKeyVerifierTriesNextKeyAfterUnsigned(t *testing.T) {
	runner := &sequentialRunner{
		results: []fakeRunResult{
			{stderr: "no matching signatures", err: errors.New("exit status 1")},
			{stdout: `[{"optional":{"keyid":"trusted"}}]`},
		},
	}
	verifier, err := NewCosignKeyVerifier(
		runner,
		registry.Credentials{},
		[]config.TrustedPublicKey{
			{Ref: "/keys/old.pub", ID: "old.pub"},
			{Ref: "/keys/current.pub", ID: "current.pub"},
		},
		false,
	)
	require.NoError(t, err)

	observation, err := verifier.Verify(context.Background(), models.DiscoveredImage{
		ID: "ghcr.io/example/api@sha256:abc",
	})
	require.NoError(t, err)
	require.Equal(t, models.StatusVerified, observation.Status)
	require.Equal(t, "current.pub", observation.Signer.KeyRef)
	require.Equal(t, 2, runner.calls)
}

func TestCosignKeyVerifierVerifyUnsigned(t *testing.T) {
	runner := &fakeRunner{
		stderr: "Error: no matching signatures found",
		err:    errors.New("exit status 1"),
	}
	verifier, err := NewCosignKeyVerifier(
		runner,
		registry.Credentials{},
		[]config.TrustedPublicKey{{Ref: "/keys/release.pub", ID: "release.pub"}},
		false,
	)
	require.NoError(t, err)

	observation, err := verifier.Verify(context.Background(), models.DiscoveredImage{
		ID: "ghcr.io/example/api@sha256:abc",
	})
	require.NoError(t, err)
	require.Equal(t, models.StatusUnsigned, observation.Status)
}

type fakeRunResult struct {
	stdout string
	stderr string
	err    error
}

type sequentialRunner struct {
	results []fakeRunResult
	calls   int
}

func (s *sequentialRunner) Run(context.Context, string, ...string) (string, string, error) {
	if s.calls >= len(s.results) {
		return "", "", errors.New("unexpected cosign invocation")
	}
	result := s.results[s.calls]
	s.calls++
	return result.stdout, result.stderr, result.err
}

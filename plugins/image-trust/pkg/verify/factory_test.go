package verify

import (
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/config"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
	"github.com/stretchr/testify/require"
)

func TestNewVerifierCompositeForBothModes(t *testing.T) {
	cfg := &config.Config{
		VerificationModes: []string{config.ModeCosignKeyless, config.ModeCosignKey},
		ModePolicy:        config.ModePolicyAny,
		TrustedIssuers:    []string{"https://token.actions.githubusercontent.com"},
		TrustedPublicKeys: []config.TrustedPublicKey{{Path: "/keys/release.pub", ID: "release.pub"}},
	}

	verifier, err := NewVerifier(cfg, &fakeRunner{}, registry.Credentials{})
	require.NoError(t, err)
	_, ok := verifier.(*CompositeVerifier)
	require.True(t, ok)
}

func TestNewVerifierSingleModeSkipsComposite(t *testing.T) {
	cfg := &config.Config{
		VerificationModes: []string{config.ModeCosignKeyless},
		TrustedIssuers:    []string{"https://token.actions.githubusercontent.com"},
	}

	verifier, err := NewVerifier(cfg, &fakeRunner{}, registry.Credentials{})
	require.NoError(t, err)
	_, ok := verifier.(*CosignVerifier)
	require.True(t, ok)
}

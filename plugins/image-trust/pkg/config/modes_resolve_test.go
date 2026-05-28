package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveVerificationModesAppendsKeylessAttestation(t *testing.T) {
	cfg := &Config{
		VerificationModes: []string{ModeCosignKeyless},
		AttestationsEnabled: true,
		AttestationTypes:    []string{"slsaprovenance1"},
		TrustedIssuers:      []string{"https://token.actions.githubusercontent.com"},
	}
	cfg.ResolveVerificationModes()
	require.Contains(t, cfg.VerificationModes, ModeCosignAttestationKeyless)
}

func TestResolveVerificationModesFromTypesOnly(t *testing.T) {
	cfg := &Config{
		VerificationModes: []string{ModeCosignKeyless},
		AttestationTypes:    []string{"spdxjson"},
		TrustedSubjects:     []string{"https://github.com/example/workflow"},
	}
	cfg.ResolveVerificationModes()
	require.Contains(t, cfg.VerificationModes, ModeCosignAttestationKeyless)
}

func TestResolveVerificationModesAppendsKeyedAttestation(t *testing.T) {
	cfg := &Config{
		VerificationModes:   []string{ModeCosignKey, ModeCosignKeyless},
		AttestationsEnabled: true,
		AttestationTypes:    []string{"cyclonedx"},
		TrustedIssuers:      []string{"https://token.actions.githubusercontent.com"},
		PublicKeyDir:        "/etc/keys",
	}
	cfg.ResolveVerificationModes()
	require.Contains(t, cfg.VerificationModes, ModeCosignAttestationKey)
	require.Contains(t, cfg.VerificationModes, ModeCosignAttestationKeyless)
}

func TestResolveVerificationModesDoesNotAppendKeyedAttestationWithoutCosignKey(t *testing.T) {
	cfg := &Config{
		VerificationModes:   []string{ModeCosignKeyless},
		AttestationsEnabled: true,
		AttestationTypes:    []string{"slsaprovenance1"},
		TrustedIssuers:      []string{"https://token.actions.githubusercontent.com"},
		PublicKeyDir:        "/etc/keys",
	}
	cfg.ResolveVerificationModes()
	require.Contains(t, cfg.VerificationModes, ModeCosignAttestationKeyless)
	require.NotContains(t, cfg.VerificationModes, ModeCosignAttestationKey)
}

func TestResolveVerificationModesDoesNotAppendKeylessAttestationWithoutCosignKeyless(t *testing.T) {
	cfg := &Config{
		VerificationModes:   []string{ModeCosignKey},
		AttestationsEnabled: true,
		AttestationTypes:    []string{"spdxjson"},
		PublicKeyDir:        "/etc/keys",
		TrustedIssuers:      []string{"https://token.actions.githubusercontent.com"},
	}
	cfg.ResolveVerificationModes()
	require.Contains(t, cfg.VerificationModes, ModeCosignAttestationKey)
	require.NotContains(t, cfg.VerificationModes, ModeCosignAttestationKeyless)
}

package verify

import (
	"fmt"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/config"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
)

// NewVerifier creates a verifier for all configured verification modes.
func NewVerifier(cfg *config.Config, runner CommandRunner, registryCreds registry.Credentials) (Verifier, error) {
	if len(cfg.VerificationModes) == 0 {
		return nil, fmt.Errorf("no verification modes configured")
	}

	verifiers := make([]Verifier, 0, len(cfg.VerificationModes))
	for _, mode := range cfg.VerificationModes {
		switch mode {
		case config.ModeCosignKeyless:
			verifier, err := NewCosignVerifier(
				runner,
				registryCreds,
				cfg.TrustedIssuers,
				cfg.TrustedSubjects,
				cfg.TrustedSubjectREs,
			)
			if err != nil {
				return nil, err
			}
			verifiers = append(verifiers, verifier)
		case config.ModeCosignKey:
			verifier, err := NewCosignKeyVerifier(runner, registryCreds, cfg.TrustedPublicKeys, cfg.IgnoreTlog)
			if err != nil {
				return nil, err
			}
			verifiers = append(verifiers, verifier)
		case config.ModeCosignAttestationKeyless:
			verifier, err := NewCosignAttestationVerifier(
				runner,
				registryCreds,
				cfg.AttestationTypes,
				cfg.TrustedIssuers,
				cfg.TrustedSubjects,
				cfg.TrustedSubjectREs,
			)
			if err != nil {
				return nil, err
			}
			verifiers = append(verifiers, verifier)
		case config.ModeCosignAttestationKey:
			verifier, err := NewCosignAttestationKeyVerifier(
				runner,
				registryCreds,
				cfg.TrustedPublicKeys,
				cfg.AttestationTypes,
				cfg.IgnoreTlog,
			)
			if err != nil {
				return nil, err
			}
			verifiers = append(verifiers, verifier)
		default:
			return nil, fmt.Errorf("unsupported verification mode %q", mode)
		}
	}

	if len(verifiers) == 1 {
		return verifiers[0], nil
	}
	return NewCompositeVerifier(cfg.ModePolicy, verifiers...)
}

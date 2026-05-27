package verify

import (
	"fmt"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/config"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
)

// NewVerifier creates a verifier for the first configured verification mode.
func NewVerifier(cfg *config.Config, runner CommandRunner, registryCreds registry.Credentials) (Verifier, error) {
	if len(cfg.VerificationModes) == 0 {
		return nil, fmt.Errorf("no verification modes configured")
	}

	switch cfg.VerificationModes[0] {
	case "cosign-keyless":
		return NewCosignVerifier(
			runner,
			registryCreds,
			cfg.TrustedIssuers,
			cfg.TrustedSubjects,
			cfg.TrustedSubjectREs,
		)
	default:
		return nil, fmt.Errorf("unsupported verification mode %q", cfg.VerificationModes[0])
	}
}

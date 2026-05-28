package verify

import (
	"context"
	"fmt"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/config"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
)

// CosignKeyVerifier verifies signatures using trusted static public keys.
type CosignKeyVerifier struct {
	runner        CommandRunner
	registryCreds registry.Credentials
	publicKeys    []config.TrustedPublicKey
	ignoreTlog    bool
}

// NewCosignKeyVerifier creates a verifier for cosign static public key signatures.
func NewCosignKeyVerifier(
	runner CommandRunner,
	registryCreds registry.Credentials,
	publicKeys []config.TrustedPublicKey,
	ignoreTlog bool,
) (*CosignKeyVerifier, error) {
	if len(publicKeys) == 0 {
		return nil, fmt.Errorf("at least one trusted public key is required")
	}
	return &CosignKeyVerifier{
		runner:        runner,
		registryCreds: registryCreds,
		publicKeys:    append([]config.TrustedPublicKey(nil), publicKeys...),
		ignoreTlog:    ignoreTlog,
	}, nil
}

func (v *CosignKeyVerifier) Name() models.VerificationMode {
	return models.VerificationModeCosignKey
}

func (v *CosignKeyVerifier) Verify(ctx context.Context, image models.DiscoveredImage) (models.VerificationObservation, error) {
	ref := image.VerificationReference()
	if ref == "" {
		return models.VerificationObservation{
			Mode:   v.Name(),
			Status: models.StatusUnknown,
			Reason: "image could not be resolved to an immutable digest reference",
		}, nil
	}

	var attempts []models.VerificationObservation
	for _, key := range v.publicKeys {
		observation, err := v.verifyWithKey(ctx, ref, key)
		if err != nil {
			return models.VerificationObservation{}, err
		}
		if observation.Status == models.StatusVerified {
			return observation, nil
		}
		attempts = append(attempts, observation)
	}

	merged := mergeObservations(attempts)
	merged.Mode = v.Name()
	return merged, nil
}

func (v *CosignKeyVerifier) verifyWithKey(ctx context.Context, ref string, key config.TrustedPublicKey) (models.VerificationObservation, error) {
	args := []string{
		"verify",
		"--output", "json",
		"--key", key.Path,
	}
	if v.ignoreTlog {
		args = append(args, "--insecure-ignore-tlog")
	}
	args = append(args, v.registryCreds.CosignArgs()...)
	args = append(args, ref)

	stdout, stderr, err := v.runner.Run(ctx, "cosign", args...)
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return models.VerificationObservation{
				Mode:   v.Name(),
				Status: models.StatusVerificationError,
				Reason: "cosign binary not available",
			}, nil
		}
		status, reason := classifyCosignFailure(firstNonEmpty(stderr, stdout, err.Error()))
		return models.VerificationObservation{
			Mode:   v.Name(),
			Status: status,
			Reason: reason,
		}, nil
	}

	signers, err := extractCosignSigners(stdout)
	if err != nil {
		return models.VerificationObservation{
			Mode:   v.Name(),
			Status: models.StatusVerificationError,
			Reason: fmt.Sprintf("cosign verification succeeded but signer data could not be parsed: %v", err),
		}, nil
	}

	signer := models.SignerDetails{KeyRef: key.ID}
	if len(signers) > 0 {
		signer.Issuer = signers[0].Issuer
		signer.Subject = signers[0].Subject
	}

	return models.VerificationObservation{
		Mode:    v.Name(),
		Status:  models.StatusVerified,
		Reason:  fmt.Sprintf("cosign verification succeeded with trusted public key %s", key.ID),
		Signer:  signer,
		Signers: signers,
	}, nil
}

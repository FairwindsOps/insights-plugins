package verify

import (
	"context"
	"fmt"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/config"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
)

// CosignAttestationKeyVerifier verifies attestations with trusted static public keys.
type CosignAttestationKeyVerifier struct {
	runner           CommandRunner
	registryCreds    registry.Credentials
	publicKeys       []config.TrustedPublicKey
	attestationTypes []string
	ignoreTlog       bool
}

// NewCosignAttestationKeyVerifier creates a keyed attestation verifier.
func NewCosignAttestationKeyVerifier(
	runner CommandRunner,
	registryCreds registry.Credentials,
	publicKeys []config.TrustedPublicKey,
	attestationTypes []string,
	ignoreTlog bool,
) (*CosignAttestationKeyVerifier, error) {
	if len(publicKeys) == 0 {
		return nil, fmt.Errorf("at least one trusted public key is required")
	}
	if len(attestationTypes) == 0 {
		return nil, fmt.Errorf("at least one attestation type is required")
	}
	return &CosignAttestationKeyVerifier{
		runner:           runner,
		registryCreds:    registryCreds,
		publicKeys:       append([]config.TrustedPublicKey(nil), publicKeys...),
		attestationTypes: append([]string(nil), attestationTypes...),
		ignoreTlog:       ignoreTlog,
	}, nil
}

func (v *CosignAttestationKeyVerifier) Name() models.VerificationMode {
	return models.VerificationModeCosignAttestationKey
}

func (v *CosignAttestationKeyVerifier) Verify(ctx context.Context, image models.DiscoveredImage) (models.VerificationObservation, error) {
	ref := v.registryCreds.VerificationReference(image.VerificationReference())
	if ref == "" {
		return models.VerificationObservation{
			Mode:   v.Name(),
			Status: models.StatusUnknown,
			Reason: "image could not be resolved to an immutable digest reference",
		}, nil
	}

	var attempts []models.VerificationObservation
	for _, key := range v.publicKeys {
		for _, attestationType := range v.attestationTypes {
			observation, err := v.verifyWithKey(ctx, ref, key, attestationType)
			if err != nil {
				return models.VerificationObservation{}, err
			}
			if observation.Status == models.StatusVerified {
				return observation, nil
			}
			attempts = append(attempts, observation)
		}
	}

	merged := mergeObservations(attempts)
	merged.Mode = v.Name()
	return merged, nil
}

func (v *CosignAttestationKeyVerifier) verifyWithKey(
	ctx context.Context,
	ref string,
	key config.TrustedPublicKey,
	attestationType string,
) (models.VerificationObservation, error) {
	args := []string{
		"verify-attestation",
		"--output", "json",
		"--type", attestationType,
		"--key", key.Ref,
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
			Reason: fmt.Sprintf("cosign attestation verification succeeded but signer data could not be parsed: %v", err),
		}, nil
	}

	signer := models.SignerDetails{KeyRef: key.ReportKeyRef()}
	if len(signers) > 0 {
		signer.Issuer = signers[0].Issuer
		signer.Subject = signers[0].Subject
	}

	return models.VerificationObservation{
		Mode:            v.Name(),
		Status:          models.StatusVerified,
		Reason:          fmt.Sprintf("cosign attestation verification succeeded with trusted public key %s for type %s", key.ReportKeyRef(), attestationType),
		AttestationType: attestationType,
		Signer:          signer,
		Signers:         signers,
	}, nil
}

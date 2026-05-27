package verify

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
)

// Verifier checks trust evidence for an immutable image reference.
type Verifier interface {
	Name() models.VerificationMode
	Verify(ctx context.Context, image models.DiscoveredImage) (models.VerificationObservation, error)
}

// CommandRunner runs external commands for verifier implementations.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (stdout string, stderr string, err error)
}

// ExecRunner runs commands using os/exec.
type ExecRunner struct{}

// Run executes a command and captures stdout and stderr.
func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// VerifyImages applies a verifier to all discovered images and returns final image results.
func VerifyImages(ctx context.Context, images []models.DiscoveredImage, verifier Verifier) ([]models.ImageTrustResult, error) {
	results := make([]models.ImageTrustResult, 0, len(images))
	for _, image := range images {
		observation, err := verifier.Verify(ctx, image)
		if err != nil {
			return nil, fmt.Errorf("verifying image %s: %w", image.Name, err)
		}
		results = append(results, models.ImageTrustResult{
			Name:             image.Name,
			ID:               image.ID,
			PullRef:          image.PullRef,
			Status:           observation.Status,
			Reason:           observation.Reason,
			VerificationMode: string(observation.Mode),
			Allowlisted:      false,
			Owners:           image.Owners,
			Signer:           observation.Signer,
			CandidateSigners: append([]models.SignerDetails(nil), observation.Signers...),
		})
	}
	return results, nil
}

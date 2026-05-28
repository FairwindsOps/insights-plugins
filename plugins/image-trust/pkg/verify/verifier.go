package verify

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
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
type ExecRunner struct {
	ExtraEnv []string
}

// Run executes a command and captures stdout and stderr.
func (r ExecRunner) Run(ctx context.Context, name string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), r.ExtraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// VerifyImages applies a verifier to all discovered images and returns final image results.
func VerifyImages(
	ctx context.Context,
	images []models.DiscoveredImage,
	creds registry.Credentials,
	verifier Verifier,
	maxConcurrent int,
	perImageTimeout time.Duration,
	retryBackoff time.Duration,
	retryJitter bool,
	verifyRetries int,
) ([]models.ImageTrustResult, error) {
	if len(images) == 0 {
		return nil, nil
	}
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}

	results := make([]models.ImageTrustResult, len(images))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrent)
	errCh := make(chan error, 1)

	for i, image := range images {
		wg.Add(1)
		sem <- struct{}{}
		go func(index int, img models.DiscoveredImage) {
			defer wg.Done()
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				if len(errCh) == 0 {
					errCh <- ctx.Err()
				}
				return
			default:
			}

			imageCtx, cancel := context.WithTimeout(ctx, perImageTimeout)
			defer cancel()

			var observation models.VerificationObservation
			var err error
			if preflight, stop := Preflight(img, creds); stop {
				observation = preflight
				observation.Mode = verifier.Name()
			} else {
				observation, err = VerifyWithRetries(imageCtx, verifier, img, verifyRetries, retryBackoff, retryJitter)
			}
			if err != nil {
				if len(errCh) == 0 {
					errCh <- fmt.Errorf("verifying image %s: %w", img.Name, err)
				}
				return
			}
			verifiedBy := string(observation.VerifiedBy)
			if verifiedBy == "" {
				verifiedBy = string(observation.Mode)
			}
			results[index] = models.ImageTrustResult{
				Name:               img.Name,
				ID:                 img.ID,
				PullRef:            img.PullRef,
				Status:             observation.Status,
				Reason:             observation.Reason,
				VerificationMode:   string(observation.Mode),
				VerifiedBy:         verifiedBy,
				AttestationType:    observation.AttestationType,
				Allowlisted:        false,
				Owners:             img.Owners,
				Signer:             observation.Signer,
				CandidateSigners:   append([]models.SignerDetails(nil), observation.Signers...),
				DigestResolveError: img.DigestResolveError,
			}
		}(i, image)
	}

	wg.Wait()
	close(errCh)

	if err := <-errCh; err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

package verify

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
	"github.com/sirupsen/logrus"
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
	var processed atomic.Int32
	total := len(images)

	logProgress := func(completed int) {
		if completed == total || completed%10 == 0 || total <= 10 {
			logrus.Infof("processed image trust checks for %d/%d images", completed, total)
		}
	}

	for i, image := range images {
		wg.Add(1)
		sem <- struct{}{}
		go func(index int, img models.DiscoveredImage) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				logProgress(int(processed.Add(1)))
			}()

			if err := ctx.Err(); err != nil {
				results[index] = imageTrustResultFromError(img, err)
				return
			}

			imageCtx, cancel := context.WithTimeout(ctx, perImageTimeout)
			defer cancel()

			var observation models.VerificationObservation
			var err error
			if preflight, stop := Preflight(img, creds); stop {
				observation = preflight
			} else {
				observation, err = VerifyWithRetries(imageCtx, verifier, img, verifyRetries, retryBackoff, retryJitter)
			}
			if err != nil {
				results[index] = imageTrustResultFromError(img, err)
				return
			}
			results[index] = imageTrustResultFromObservation(img, observation)
		}(i, image)
	}

	wg.Wait()

	return results, nil
}

func imageTrustResultFromObservation(img models.DiscoveredImage, observation models.VerificationObservation) models.ImageTrustResult {
	verifiedBy := string(observation.VerifiedBy)
	if verifiedBy == "" {
		verifiedBy = string(observation.Mode)
	}
	return models.ImageTrustResult{
		Name:               img.Name,
		ID:                 img.ID,
		PullRef:            img.PullRef,
		Status:             observation.Status,
		Reason:             observation.Reason,
		VerificationMode:   verificationModeFromObservation(observation),
		VerifiedBy:         verifiedBy,
		AttestationType:    observation.AttestationType,
		Allowlisted:        false,
		Owners:             img.Owners,
		Signer:             observation.Signer,
		CandidateSigners:   append([]models.SignerDetails(nil), observation.Signers...),
		DigestResolveError: img.DigestResolveError,
	}
}

func imageTrustResultFromError(img models.DiscoveredImage, err error) models.ImageTrustResult {
	reason := err.Error()
	if reason == "" {
		reason = "image trust verification failed"
	}
	return models.ImageTrustResult{
		Name:               img.Name,
		ID:                 img.ID,
		PullRef:            img.PullRef,
		Status:             models.StatusVerificationError,
		Reason:             reason,
		Allowlisted:        false,
		Owners:             img.Owners,
		DigestResolveError: img.DigestResolveError,
	}
}

func verificationModeFromObservation(observation models.VerificationObservation) string {
	if observation.VerifiedBy != "" {
		return string(observation.VerifiedBy)
	}
	if observation.Mode != "" {
		return string(observation.Mode)
	}
	return ""
}

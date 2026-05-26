package policy

import (
	"fmt"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
)

// AllowlistMatcher checks whether images should be exempted from findings.
type AllowlistMatcher struct {
	imagePatterns    []string
	registryPatterns []string
}

// NewAllowlistMatcher creates an allowlist matcher from configured patterns.
func NewAllowlistMatcher(imagePatterns, registryPatterns []string) *AllowlistMatcher {
	return &AllowlistMatcher{
		imagePatterns:    append([]string(nil), imagePatterns...),
		registryPatterns: append([]string(nil), registryPatterns...),
	}
}

// Apply marks results as allowlisted when an allowlist rule matches.
func (m *AllowlistMatcher) Apply(images []models.DiscoveredImage, results []models.ImageTrustResult) ([]models.ImageTrustResult, error) {
	if len(images) != len(results) {
		return nil, fmt.Errorf("images and results length mismatch: %d != %d", len(images), len(results))
	}

	updated := make([]models.ImageTrustResult, len(results))
	copy(updated, results)
	for i := range updated {
		match, reason, err := m.match(images[i], updated[i])
		if err != nil {
			return nil, err
		}
		if match {
			updated[i].Allowlisted = true
			updated[i].AllowlistReason = reason
		}
	}
	return updated, nil
}

func (m *AllowlistMatcher) match(image models.DiscoveredImage, result models.ImageTrustResult) (bool, string, error) {
	imageCandidates := []string{
		image.Name,
		image.ID,
		image.PullRef,
		image.VerificationReference(),
	}
	for _, pattern := range m.imagePatterns {
		for _, candidate := range imageCandidates {
			if candidate == "" {
				continue
			}
			matched, err := doublestar.Match(pattern, candidate)
			if err != nil {
				return false, "", fmt.Errorf("matching image allowlist pattern %q: %w", pattern, err)
			}
			if matched {
				return true, "image allowlist matched: " + pattern, nil
			}
		}
	}

	registryCandidates := []string{
		registryFromReference(image.Name),
		registryFromReference(image.VerificationReference()),
		registryFromReference(result.ID),
	}
	for _, pattern := range m.registryPatterns {
		for _, candidate := range registryCandidates {
			if candidate == "" {
				continue
			}
			matched, err := doublestar.Match(pattern, candidate)
			if err != nil {
				return false, "", fmt.Errorf("matching registry allowlist pattern %q: %w", pattern, err)
			}
			if matched {
				return true, "registry allowlist matched: " + pattern, nil
			}
		}
	}

	return false, "", nil
}

func registryFromReference(ref string) string {
	if ref == "" {
		return ""
	}
	candidate := ref
	if idx := strings.Index(candidate, "@"); idx >= 0 {
		candidate = candidate[:idx]
	}
	lastSlash := strings.LastIndex(candidate, "/")
	lastColon := strings.LastIndex(candidate, ":")
	if lastColon > lastSlash {
		candidate = candidate[:lastColon]
	}

	firstSegment := candidate
	if slash := strings.Index(firstSegment, "/"); slash >= 0 {
		firstSegment = firstSegment[:slash]
	}

	if firstSegment == "localhost" || strings.Contains(firstSegment, ".") || strings.Contains(firstSegment, ":") {
		return firstSegment
	}
	return "docker.io"
}

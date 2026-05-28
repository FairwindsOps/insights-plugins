package registry

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sirupsen/logrus"
)

// ResolveDigest looks up the registry manifest digest for a tag-only image reference.
func ResolveDigest(ctx context.Context, creds Credentials, image models.DiscoveredImage) (string, error) {
	ref := tagReference(image)
	if ref == "" {
		return "", fmt.Errorf("image %q is not a tag reference", image.Name)
	}

	if creds.CertDir != "" {
		_ = os.Setenv("SSL_CERT_DIR", creds.CertDir)
	}

	parsed, err := name.ParseReference(ref, name.WeakValidation)
	if err != nil {
		return "", fmt.Errorf("parsing reference %q: %w", ref, err)
	}

	keychain, err := creds.Keychain()
	if err != nil {
		return "", err
	}

	desc, err := remote.Head(parsed, remote.WithContext(ctx), remote.WithAuthFromKeychain(keychain))
	if err != nil {
		return "", err
	}

	digestRef, err := name.NewDigest(fmt.Sprintf("%s@%s", parsed.Context().Name(), desc.Digest.String()))
	if err != nil {
		return "", err
	}
	return digestRef.String(), nil
}

func tagReference(image models.DiscoveredImage) string {
	candidates := []string{image.Name, image.PullRef}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if strings.Contains(candidate, "@sha256:") {
			continue
		}
		if strings.HasPrefix(candidate, "sha256:") {
			continue
		}
		return candidate
	}
	return ""
}

// EnrichDigest resolves tag-only images to digest-backed references when needed.
func EnrichDigest(ctx context.Context, creds Credentials, image models.DiscoveredImage) models.DiscoveredImage {
	if image.VerificationReference() != "" {
		return image
	}
	ref := tagReference(image)
	if ref == "" {
		return image
	}

	digestRef, err := ResolveDigest(ctx, creds, image)
	if err != nil {
		logrus.Warnf("could not resolve digest for %s: %v", ref, err)
		return image
	}

	logrus.Infof("resolved digest for %s -> %s", ref, digestRef)
	image.ID = digestRef
	if image.PullRef == "" || !strings.Contains(image.PullRef, "@sha256:") {
		image.PullRef = digestRef
	}
	return image
}

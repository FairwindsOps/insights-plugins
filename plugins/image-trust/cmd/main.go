package main

import (
	"context"
	"strings"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/config"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/discovery"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/output"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/policy"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/report"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/resolve"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/sigstore"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/verify"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

func main() {
	ctx := context.Background()

	cfg, err := config.LoadFromEnvironment()
	if err != nil {
		logrus.Fatalf("loading config: %v", err)
	}
	logrus.Infof("verification modes: %s (policy=%s)", strings.Join(cfg.VerificationModes, ","), cfg.ModePolicy)

	kubeClient, err := kubernetesClient()
	if err != nil {
		logrus.Fatalf("creating kubernetes client: %v", err)
	}

	discoveryResult, err := discovery.ListImages(ctx, kubeClient, cfg.NamespaceBlocklist, cfg.NamespaceAllowlist)
	if err != nil {
		logrus.Fatalf("discovering images: %v", err)
	}

	prepared, err := registry.Prepare(ctx, cfg)
	if err != nil {
		logrus.Fatalf("preparing registry credentials: %v", err)
	}
	defer prepared.Cleanup()

	logrus.Infof("discovered %d images", len(discoveryResult.Images))
	images := resolve.Images(ctx, prepared.Credentials, discoveryResult.Images, cfg.ResolveDigests, cfg.MaxConcurrentScans)

	results, err := verifyImages(ctx, cfg, prepared.Credentials, images, time.Now())
	if err != nil {
		logrus.Fatalf("verifying images: %v", err)
	}

	finalReport := report.Build(results, report.PolicyFromAllowlists(
		cfg.ImageAllowlist,
		cfg.RegistryAllowlist,
		cfg.SignerAllowlist,
	))
	if err := output.WriteFinalReport(finalReport); err != nil {
		logrus.Fatalf("writing report: %v", err)
	}

	logrus.Infof("wrote image trust report to %s", output.OutputFile)
}

func kubernetesClient() (kubernetes.Interface, error) {
	kubeConfig, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(kubeConfig)
}

func verifyImages(ctx context.Context, cfg *config.Config, registryCreds registry.Credentials, images []models.DiscoveredImage, now time.Time) ([]models.ImageTrustResult, error) {
	sigstoreEnv, err := sigstore.ExtraEnv()
	if err != nil {
		return nil, err
	}
	runner := verify.ExecRunner{ExtraEnv: registryCreds.ExtraEnv(sigstoreEnv...)}
	verifier, err := verify.NewVerifier(cfg, runner, registryCreds)
	if err != nil {
		return nil, err
	}

	results, err := verify.VerifyImages(
		ctx,
		images,
		registryCreds,
		verifier,
		cfg.MaxConcurrentScans,
		cfg.ImageVerifyTimeout,
		cfg.VerifyRetryBackoff,
		cfg.VerifyRetryJitter,
		cfg.VerifyRetries,
	)
	if err != nil {
		return nil, err
	}

	matcher := policy.NewAllowlistMatcher(cfg.ImageAllowlist, cfg.RegistryAllowlist, cfg.SignerAllowlist)
	results, err = matcher.Apply(images, results)
	if err != nil {
		return nil, err
	}

	for i := range results {
		results[i].LastCheckedAt = now.UTC()
	}
	return results, nil
}

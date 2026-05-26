package main

import (
	"context"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/config"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/discovery"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/output"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/report"
	"github.com/sirupsen/logrus"
)

func main() {
	ctx := context.Background()

	cfg, err := config.LoadFromEnvironment()
	if err != nil {
		logrus.Fatalf("loading config: %v", err)
	}

	images, err := discovery.ListImages(ctx, cfg.NamespaceBlocklist, cfg.NamespaceAllowlist)
	if err != nil {
		logrus.Fatalf("discovering images: %v", err)
	}

	logrus.Infof("discovered %d images", len(images))

	finalReport := report.Build(images, time.Now())
	if err := output.WriteReport(output.FinalReportPath, finalReport); err != nil {
		logrus.Fatalf("writing report: %v", err)
	}

	logrus.Infof("wrote image trust report to %s", output.FinalReportPath)
}

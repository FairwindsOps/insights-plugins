package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/config"
	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/image"
	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/util"
	"github.com/sirupsen/logrus"
)

const outputFile = image.TempDir + "/final-report.json"

/*
 * Downloads the latest trivy report containing  all cluster scans and recommendation
 * Reads all images from the cluster and scan few of them
 * Uploads another report with the images scanned and not scanned from the latest report
 * Images or images recommendations that no longer belongs to that cluster are filtered out
 */
func main() {
	ctx := context.TODO()
	setLogLevel(os.Getenv("LOGRUS_LEVEL"))
	cfg, err := config.LoadFromEnvironment()
	if err != nil {
		logrus.Fatalf("could not set environment variables: %v", err)
	}

	logrus.Debugf("config is %#v", *cfg)

	err = util.CheckEnvironmentVariables()
	if err != nil {
		logrus.Fatal("error checking environment variables: ", err)
	}

	if !cfg.Offline {
		_, err := util.RunCommand(exec.Command("trivy", "image", "--download-db-only"), "downloading trivy database")
		if err != nil {
			logrus.Fatal(err)
		}
	}

	host := os.Getenv("FAIRWINDS_INSIGHTS_HOST")
	org := os.Getenv("FAIRWINDS_ORG")
	cluster := os.Getenv("FAIRWINDS_CLUSTER")
	token := os.Getenv("FAIRWINDS_TOKEN")
	noRecommendations := os.Getenv("NO_RECOMMENDATIONS")

	lastReport, err := image.FetchLastReport(ctx, host, org, cluster, token)
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("Latest report has %d images", len(lastReport.Images))
	for _, i := range lastReport.Images {
		logrus.Debugf("%v - %v", i.Name, i.ID)
	}

	logrus.Infof("%d namespaces allowed, %d namespaces blocked", len(cfg.NamespaceAllowlist), len(cfg.NamespaceBlocklist))
	inClusterImages, err := image.GetImages(ctx, cfg.NamespaceBlocklist, cfg.NamespaceAllowlist)
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("Found %d images in cluster", len(inClusterImages))
	for _, i := range inClusterImages {
		logrus.Debugf("%v - %v", i.Name, i.ID)
	}

	imagesToScan := image.GetUnscannedImagesToScan(inClusterImages, lastReport.Images, cfg.NumberToScan)
	unscannedCount := len(imagesToScan)
	logrus.Infof("Found %d images that have never been scanned", unscannedCount)
	imagesToScan = image.GetImagesToReScan(inClusterImages, *lastReport, imagesToScan, cfg.NumberToScan)
	logrus.Infof("Will re-scan %d additional images", len(imagesToScan)-unscannedCount)
	for _, i := range imagesToScan {
		logrus.Debugf("%v - %v", i.Name, i.ID)
	}

	// Owners info from latest report might be out-of-date, we need to update it using the cluster info
	lastReport.Images = image.UpdateOwnersReferenceOnMatchingImages(lastReport.Images, inClusterImages)
	// Remove any images from the report that are no longer in the cluster
	lastReport.Images = image.GetMatchingImages(lastReport.Images, inClusterImages, false)
	logrus.Infof("%d images after removing images no longer in cluster", len(lastReport.Images))
	// Remove any images from the report that we're going to re-scan now
	lastReport.Images = image.GetUnmatchingImages(lastReport.Images, imagesToScan, false)
	logrus.Infof("%d images after removing images to be scanned", len(lastReport.Images))
	// Remove any recommendations from the report that no longer have a corresponding image in the cluster
	lastReport.Images = image.GetMatchingImages(lastReport.Images, inClusterImages, true)
	logrus.Infof("%d images after removing recommendations that don't match", len(lastReport.Images))

	if cfg.HasGKESAAnnotation {
		// this command should be run before trivy and skopeo commands
		// it configures ~/.docker/config.json with registries and access tokens
		_, err := util.RunCommand(exec.Command("gcloud", "-q", "auth", "configure-docker"), "setting up gcloud docker authentication")
		if err != nil {
			logrus.Fatalf("could not get gcloud access token: %v", err)
		}
	}

	logrus.Infof("Starting image scans")
	allReports := image.ScanImages(image.ScanImage, imagesToScan, cfg.MaxConcurrentScans, cfg.ExtraFlags)

	if noRecommendations == "" {
		logrus.Infof("Scanning recommendations")
		recommendationsToScan := image.GetNewestVersionsToScan(ctx, allReports, imagesToScan)
		// Remove any recommendations from the report that we're going to re-scan now
		lastReport.Images = image.GetUnmatchingImages(lastReport.Images, recommendationsToScan, true)
		logrus.Infof("%d images after removing recommendations that will be scanned", len(lastReport.Images))
		logrus.Infof("Scanning %d recommended images", len(recommendationsToScan))
		recommendationReport := image.ScanImages(image.ScanImage, recommendationsToScan, cfg.MaxConcurrentScans, cfg.ExtraFlags)
		logrus.Infof("Done scanning recommendations")
		allReports = append(allReports, recommendationReport...)
	}
	logrus.Infof("Done with all scans, minimizing report")
	minimizedReport := image.Minimize(allReports, *lastReport)
	data, err := json.Marshal(minimizedReport)
	if err != nil {
		logrus.Fatalf("could not marshal report: %v", err)
	}
	logrus.Infof("Writing to file %s", outputFile)
	err = os.WriteFile(outputFile, data, 0644)
	if err != nil {
		logrus.Fatalf("could not write to output file: %v", err)
	}
	logrus.Info("Finished writing file ", outputFile)
}

func setLogLevel(logLevel string) {
	if logLevel != "" {
		lvl, err := logrus.ParseLevel(logLevel)
		if err != nil {
			panic(fmt.Errorf("Invalid log level %q (should be one of trace, debug, info, warning, error, fatal, panic), error: %v", logLevel, err))
		}
		logrus.SetLevel(lvl)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}

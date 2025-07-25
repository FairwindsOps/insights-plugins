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

	err = util.CheckRequiredEnvironmentVariables()
	if err != nil {
		logrus.Fatal("error checking environment variables: ", err)
	}

	if !cfg.Offline && cfg.TrivyServerURL == "" {
		err := updateTrivyDatabases()
		if err != nil {
			logrus.Fatalf("could not update trivy database: %v", err)
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

	if len(cfg.ImagesToScan) > 0 {
		logrus.Infof("Images to scan is set on the environment, we will only scan those images")
		inClusterImages = util.FilterImagesByName(inClusterImages, cfg.ImagesToScan)
	}

	imagesToScan := image.GetUnscannedImagesToScan(inClusterImages, lastReport.Images, cfg.MaxImagesToScan)
	unscannedCount := len(imagesToScan)
	logrus.Infof("Found %d images that have never been scanned", unscannedCount)
	imagesToScan = image.GetImagesToReScan(inClusterImages, *lastReport, imagesToScan, cfg.MaxImagesToScan)
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

	registryOAuth2AccessTokenMap := map[string]string{}
	if cfg.HasGKESAAnnotation {
		// it seems that AWS IRSA is support but not GKE SA, so we need to get the token manually and inject into skopeo
		oauth2AccessToken, err := util.RunCommand(exec.Command("gcloud", "auth", "print-access-token"), "getting gcloud access token")
		if err != nil {
			logrus.Fatalf("could not get gcloud access token: %v", err)
		}
		registryOAuth2AccessTokenMap["gcr.io"] = oauth2AccessToken
		registryOAuth2AccessTokenMap["docker.pkg.dev"] = oauth2AccessToken
	}

	logrus.Infof("Starting image scans")
	allReports := image.ScanImages(image.ScanImage, imagesToScan, cfg.MaxConcurrentScans, cfg.ExtraFlags, cfg.TrivyServerURL, registryOAuth2AccessTokenMap)

	if noRecommendations == "" {
		logrus.Infof("Scanning recommendations")
		recommendationsToScan := image.GetNewestVersionsToScan(ctx, allReports, imagesToScan, registryOAuth2AccessTokenMap)
		// Remove any recommendations from the report that we're going to re-scan now
		lastReport.Images = image.GetUnmatchingImages(lastReport.Images, recommendationsToScan, true)
		logrus.Infof("%d images after removing recommendations that will be scanned", len(lastReport.Images))
		logrus.Infof("Scanning %d recommended images", len(recommendationsToScan))
		recommendationReport := image.ScanImages(image.ScanImage, recommendationsToScan, cfg.MaxConcurrentScans, cfg.ExtraFlags, cfg.TrivyServerURL, registryOAuth2AccessTokenMap)
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

func updateTrivyDatabases() error {
	args := []string{
		"image", "--download-db-only",
		"--db-repository", "ghcr.io/aquasecurity/trivy-db:2,public.ecr.aws/aquasecurity/trivy-db:2,docker.io/aquasec/trivy-db:2",
	}
	_, err := util.RunCommand(exec.Command("trivy", args...), "downloading trivy database")
	if err != nil {
		return fmt.Errorf("downloading trivy database: %w", err)
	}

	args = []string{
		"image", "--download-java-db-only",
		"--java-db-repository", "ghcr.io/aquasecurity/trivy-java-db:1,public.ecr.aws/aquasecurity/trivy-java-db:1,docker.io/aquasec/trivy-java-db:1",
	}
	_, err = util.RunCommand(exec.Command("trivy", args...), "downloading trivy java database")
	if err != nil {
		return fmt.Errorf("downloading trivy java database: %w", err)
	}
	return nil
}

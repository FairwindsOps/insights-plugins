package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/image"
	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/util"
	"github.com/sirupsen/logrus"
)

var maxConcurrentScans = 5
var numberToScan = 10
var extraFlags string

const outputFile = image.TempDir + "/final-report.json"

/*
 * Downloads the latest trivy report containing  all cluster scans and recommendation
 * Reads all images from the cluster and scan few of them
 * Uploads another report with the images scanned and not scanned from the latest report
 * Images or images recommendations that no longer belongs to that cluster are filtered out
 */
func main() {
	setLogLevel(os.Getenv("LOGRUS_LEVEL"))
	setEnv()

	host := os.Getenv("FAIRWINDS_INSIGHTS_HOST")
	org := os.Getenv("FAIRWINDS_ORG")
	cluster := os.Getenv("FAIRWINDS_CLUSTER")
	token := os.Getenv("FAIRWINDS_TOKEN")
	noRecommendations := os.Getenv("NO_RECOMMENDATIONS")

	lastReport, err := image.GetLastReport(host, org, cluster, token)
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("Latest report has %d images", len(lastReport.Images))
	for _, i := range lastReport.Images {
		logrus.Debugf("%v - %v", i.Name, i.ID)
	}
	ctx := context.Background()
	namespaceBlocklist, namespaceAllowlist := getNamespaceBlocklistAllowlistFromEnv()
	logrus.Infof("%d namespaces allowed, %d namespaces blocked", len(namespaceAllowlist), len(namespaceBlocklist))
	images, err := image.GetImages(ctx, namespaceBlocklist, namespaceAllowlist)
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("Found %d images in cluster", len(images))
	for _, i := range images {
		logrus.Debugf("%v - %v", i.Name, i.ID)
	}

	imagesToScan := image.GetUnscannedImagesToScan(images, lastReport.Images, numberToScan)
	unscannedCount := len(imagesToScan)
	logrus.Infof("Found %d images that have never been scanned", unscannedCount)
	imagesToScan = image.GetImagesToRescan(images, *lastReport, imagesToScan, numberToScan)
	logrus.Infof("Will rescan %d additional images", len(imagesToScan)-unscannedCount)
	for _, i := range imagesToScan {
		logrus.Debugf("%v - %v", i.Name, i.ID)
	}

	// Owners info from latest report might be out-of-date, we need to update it using the cluster info
	lastReport.Images = image.UpdateOwnersReferenceOnMatchingImages(lastReport.Images, images)
	// Remove any images from the report that are no longer in the cluster
	lastReport.Images = image.GetMatchingImages(lastReport.Images, images, false)
	logrus.Infof("%d images after removing images no longer in cluster", len(lastReport.Images))
	// Remove any images from the report that we're going to re-scan now
	lastReport.Images = image.GetUnmatchingImages(lastReport.Images, imagesToScan, false)
	logrus.Infof("%d images after removing images to be scanned", len(lastReport.Images))
	// Remove any recommendations from the report that no longer have a corresponding image in the cluster
	lastReport.Images = image.GetMatchingImages(lastReport.Images, images, true)
	logrus.Infof("%d images after removing recommendations that don't match", len(lastReport.Images))

	logrus.Infof("Starting image scans")
	allReports := image.ScanImages(imagesToScan, maxConcurrentScans, extraFlags, false)

	if noRecommendations == "" {
		logrus.Infof("Scanning recommendations")
		recommendationsToScan := image.GetNewestVersionsToScan(ctx, allReports, imagesToScan)
		// Remove any recommendations from the report that we're going to re-scan now
		lastReport.Images = image.GetUnmatchingImages(lastReport.Images, recommendationsToScan, true)
		logrus.Infof("%d images after removing recommendations that will be scanned", len(lastReport.Images))
		logrus.Infof("Scanning %d recommended images", len(recommendationsToScan))
		recommendationReport := image.ScanImages(recommendationsToScan, maxConcurrentScans, extraFlags, true)
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
	err = ioutil.WriteFile(outputFile, data, 0644)
	if err != nil {
		logrus.Fatalf("could not write to output file: %v", err)
	}
	logrus.Info("Finished writing file ", outputFile)
}

func getNamespaceBlocklistAllowlistFromEnv() ([]string, []string) {
	var namespaceBlocklist, namespaceAllowlist []string
	if os.Getenv("NAMESPACE_BLACKLIST") != "" {
		namespaceBlocklist = strings.Split(os.Getenv("NAMESPACE_BLACKLIST"), ",")
	}
	if os.Getenv("NAMESPACE_BLOCKLIST") != "" {
		namespaceBlocklist = strings.Split(os.Getenv("NAMESPACE_BLOCKLIST"), ",")
	}
	if os.Getenv("NAMESPACE_ALLOWLIST") != "" {
		namespaceAllowlist = strings.Split(os.Getenv("NAMESPACE_ALLOWLIST"), ",")
	}
	return namespaceBlocklist, namespaceAllowlist
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

func setEnv() {
	concurrencyStr := os.Getenv("MAX_CONCURRENT_SCANS")
	if concurrencyStr != "" {
		var err error
		maxConcurrentScans, err = strconv.Atoi(concurrencyStr)
		if err != nil {
			panic(err)
		}
	}

	numberToScanStr := os.Getenv("MAX_SCANS")
	if numberToScanStr != "" {
		var err error
		numberToScan, err = strconv.Atoi(numberToScanStr)
		if err != nil {
			panic(err)
		}
	}

	ignoreUnfixedStr := os.Getenv("IGNORE_UNFIXED")
	if ignoreUnfixedStr != "" {
		ignoreUnfixedBool, err := strconv.ParseBool(ignoreUnfixedStr)
		if err != nil {
			panic(err)
		}
		if ignoreUnfixedBool {
			extraFlags += "--ignore-unfixed"
		}
	}

	if os.Getenv("OFFLINE") == "" {
		err := util.RunCommand(exec.Command("trivy", "image", "--download-db-only"), "downloading trivy database")
		if err != nil {
			panic(err)
		}
	}

	err := util.CheckEnvironmentVariables()
	if err != nil {
		panic(err)
	}
}

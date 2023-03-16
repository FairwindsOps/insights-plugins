package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/image"
	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/util"
	"github.com/sirupsen/logrus"
)

var maxConcurrentScans = 5
var numberToScan = 10
var extraFlags = ""

const outputFile = image.TempDir + "/final-report.json"

/*
 * Downloads the latest trivy report containing  all cluster scans and recommendation
 * Reads all images from the cluster and scan few of them
 * Uploads another report with the images scanned and not scanned from the latest report
 * Images or images recommendations that no longer belongs to that cluster are filtered out
 */
func main() {
	setLogLevel()
	setEnv()
	lastReport, err := image.GetLastReport()
	if err != nil {
		logrus.Fatal(err)
	}
	ctx := context.Background()
	images, err := image.GetImages(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("Found %d images in cluster. All images:", len(images))
	for _, i := range images {
		logrus.Infof("%v - %v", i.Name, i.ID)
	}
	imagesToScan := image.GetUnscannedImagesToScan(images, lastReport.Images, numberToScan)
	unscannedCount := len(imagesToScan)
	logrus.Infof("Found %d images that have never been scanned", unscannedCount)
	imagesToScan = image.GetImagesToRescan(images, *lastReport, imagesToScan, numberToScan)
	logrus.Infof("Will rescan %d additional images", len(imagesToScan) - unscannedCount)
	logrus.Infof("Listing images to be scanned:")
	for _, i := range imagesToScan {
		logrus.Infof("%v - %v", i.Name, i.ID)
	}
	logrus.Infof("Latest report has %d images", len(lastReport.Images))
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

	if os.Getenv("NO_RECOMMENDATIONS") == "" {
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

func setLogLevel() {
	if os.Getenv("LOGRUS_LEVEL") != "" {
		lvl, err := logrus.ParseLevel(os.Getenv("LOGRUS_LEVEL"))
		if err != nil {
			panic(fmt.Errorf("Invalid log level %q (should be one of trace, debug, info, warning, error, fatal, panic), error: %v", os.Getenv("LOGRUS_LEVEL"), err))
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

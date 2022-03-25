package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/image"
	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/util"
	"github.com/sirupsen/logrus"
)

var maxConcurrentScans = 5
var numberToScan = 10
var extraFlags = ""

const outputFile = image.TempDir + "/final-report.json"

type newestVersions struct {
	repo     string
	versions []string
	err      error
}

func main() {
	setEnv()
	lastReport := image.GetLastReport()
	ctx := context.Background()
	images, err := image.GetImages(ctx)
	if err != nil {
		panic(err)
	}
	imagesToScan := getImagesToScan(images, lastReport.Images)
	logrus.Infof("Listing images from cluster:")
	for _, i := range images {
		logrus.Infof("%v - %v", i.ID, i.Name)
	}
	logrus.Infof("Listing images to be scanned:")
	for _, i := range imagesToScan {
		logrus.Infof("%v - %v", i.ID, i.Name)
	}
	lastReport.Images = getImagesToKeep(images, lastReport, imagesToScan)
	allReports := image.ScanImages(imagesToScan, maxConcurrentScans, extraFlags)
	newImagesToScan := getNewestVersionsToScan(ctx, allReports)
	newReport := image.ScanImages(newImagesToScan, maxConcurrentScans, extraFlags)
	aggregated := append(allReports, newReport...)
	finalReport := image.Minimize(aggregated, lastReport)
	data, err := json.Marshal(finalReport)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(outputFile, data, 0644)
	if err != nil {
		panic(err)
	}
	logrus.Info("Finished writing file ", outputFile)
}

func getNewestVersionsToScan(ctx context.Context, allReports []models.ImageReport) []models.Image {
	var imageWithVulns []models.ImageReport
	for _, report := range allReports {
		if len(report.Report) > 0 {
			imageWithVulns = append(imageWithVulns, report)
		}
	}
	versionsChan := make(chan newestVersions, len(imageWithVulns))
	for _, i := range imageWithVulns {
		go getNewestVersions(versionsChan, ctx, i)
	}
	newImagesToScan := []models.Image{}
	for i := 0; i < len(imageWithVulns); i++ {
		vc := <-versionsChan
		if vc.err == nil {
			for _, v := range vc.versions {
				newImagesToScan = append(newImagesToScan, models.Image{
					ID:                 fmt.Sprintf("%v:%v", vc.repo, v),
					Name:               fmt.Sprintf("%v:%v", vc.repo, v),
					PullRef:            fmt.Sprintf("%v:%v", vc.repo, v),
					RecommendationOnly: true,
				})
			}
		}
	}
	return newImagesToScan
}

func getNewestVersions(versionsChan chan newestVersions, ctx context.Context, img models.ImageReport) {
	parts := strings.Split(img.Name, ":")
	if len(parts) != 2 {
		return
	}
	repo := parts[0]
	tag := parts[1]
	versions, err := image.GetNewestVersions(ctx, repo, tag)
	if err != nil {
		versionsChan <- newestVersions{
			err: err,
		}
		return
	}
	versionsChan <- newestVersions{
		repo:     repo,
		versions: versions,
	}
}

func getShaFromID(id string) string {
	if len(strings.Split(id, "@")) > 1 {
		return strings.Split(id, "@")[1]
	}
	return id
}

func getImagesToScan(images []models.Image, lastReportImages []models.ImageDetailsWithRefs) []models.Image {
	imagesToScan := make([]models.Image, 0)
	for _, image := range images {
		found := false

		for _, report := range lastReportImages {
			reportSha := getShaFromID(report.ID)
			imageSha := getShaFromID(image.ID)
			if report.Name == image.Name && reportSha == imageSha {
				found = true
				break
			}
		}
		if !found {
			imagesToScan = append(imagesToScan, image)
		}
	}
	if len(imagesToScan) > numberToScan {
		imagesToScan = imagesToScan[:numberToScan]
	}
	return imagesToScan
}

func getImagesToKeep(images []models.Image, lastReport models.MinimizedReport, imagesToScan []models.Image) []models.ImageDetailsWithRefs {
	imagesToKeep := make([]models.ImageDetailsWithRefs, 0)
	sort.Slice(lastReport.Images, func(a, b int) bool {
		return lastReport.Images[a].LastScan == nil || lastReport.Images[b].LastScan != nil && lastReport.Images[a].LastScan.Before(*lastReport.Images[b].LastScan)
	})
	for _, report := range lastReport.Images {
		keep := false
		for _, image := range images {
			reportSha := getShaFromID(report.ID)
			imageSha := getShaFromID(image.ID)
			if report.Name == image.Name && reportSha == imageSha {
				if len(imagesToScan) < numberToScan {
					imagesToScan = append(imagesToScan, image)
					break
				}
				keep = true
				break
			}
		}
		if keep || report.RecommendationOnly {
			imagesToKeep = append(imagesToKeep, report)
		}
	}
	return imagesToKeep
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

	err := util.RunCommand(exec.Command("trivy", "image", "--download-db-only"), "downloading trivy database")
	if err != nil {
		panic(err)
	}
	util.CheckEnvironmentVariables()
}

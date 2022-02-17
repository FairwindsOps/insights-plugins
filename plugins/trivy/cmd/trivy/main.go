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

	"github.com/fairwindsops/insights-plugins/trivy/pkg/image"
	"github.com/fairwindsops/insights-plugins/trivy/pkg/models"
	"github.com/fairwindsops/insights-plugins/trivy/pkg/util"
)

var maxConcurrentScans = 5
var numberToScan = 10

const outputFile = image.TempDir + "/final-report.json"

func main() {

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

	err := util.RunCommand(exec.Command("trivy", "image", "--download-db-only"), "downloading trivy database")
	if err != nil {
		panic(err)
	}
	util.CheckEnvironmentVariables()
	lastReport := image.GetLastReport()
	ctx := context.Background()
	images, err := image.GetImages(ctx)
	if err != nil {
		panic(err)
	}

	imagesToScan := make([]models.Image, 0)
	for _, image := range images {
		found := false

		for _, report := range lastReport.Images {
			if report.Name == image.Name && report.ID == image.ID {
				found = true
				break
			}
		}
		if !found {
			imagesToScan = append(imagesToScan, image)
		}
	}
	imagesToKeep := make([]models.ImageDetailsWithRefs, 0)
	sort.Slice(lastReport.Images, func(a, b int) bool {
		return lastReport.Images[a].LastScan == nil || lastReport.Images[b].LastScan != nil && lastReport.Images[a].LastScan.Before(*lastReport.Images[b].LastScan)
	})
	for _, report := range lastReport.Images {
		keep := false
		for _, image := range images {
			if report.Name == image.Name && report.ID == image.ID {
				if len(imagesToScan) < numberToScan {
					imagesToScan = append(imagesToScan, image)
					break
				}
				keep = true
				break
			}
		}
		if keep {
			imagesToKeep = append(imagesToKeep, report)
		}
	}
	lastReport.Images = imagesToKeep
	if len(imagesToScan) > numberToScan {
		imagesToScan = imagesToScan[:numberToScan]
	}
	allReports := image.ScanImages(imagesToScan, maxConcurrentScans)
	finalReport := image.Minimize(allReports, lastReport)

	data, err := json.Marshal(finalReport)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(outputFile, data, 0644)
	if err != nil {
		panic(err)
	}
	var imageWithVulns []models.ImageReport
	for _, report := range allReports {
		if len(report.Report) > 0 {
			imageWithVulns = append(imageWithVulns, report)
		}
	}
	newImagesToScan := []models.Image{}
	for _, img := range imageWithVulns {
		repo := strings.Split(img.Name, ":")[0]
		tag := strings.Split(img.Name, ":")[1]
		versions, err := image.GetNewestVersions(repo, tag)
		if err != nil {
			continue
		}
		for _, v := range versions {
			newImagesToScan = append(newImagesToScan, models.Image{
				PullRef: fmt.Sprintf("%v:%v", repo, v),
			})
		}
	}
	newReport := image.ScanImages(newImagesToScan, maxConcurrentScans)
	fmt.Println("New report: ", newReport)
}

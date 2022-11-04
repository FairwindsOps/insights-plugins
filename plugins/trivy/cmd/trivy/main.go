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
	logrus.Infof("Listing images from cluster:")
	for _, i := range images {
		logrus.Infof("%v - %v", i.ID, i.Name)
	}
	imagesToScan := getUnscannedImagesToScan(images, lastReport.Images)
	imagesToScan = getImagesToRescan(images, *lastReport, imagesToScan)
	logrus.Infof("Listing images to be scanned:")
	for _, i := range imagesToScan {
		logrus.Infof("%v - %v", i.ID, i.Name)
	}
	clusterImagesToKeep := getClusterImagesToKeep(images, *lastReport, imagesToScan)
	logrus.Infof("Starting image scans")
	allReports := image.ScanImages(imagesToScan, maxConcurrentScans, extraFlags, false)
	lastReport.Images = clusterImagesToKeep
	aggregated := allReports
	if os.Getenv("NO_RECOMMENDATIONS") == "" {
		logrus.Infof("Scanning recommendations")
		recommendationsToScan := getNewestVersionsToScan(ctx, allReports, imagesToScan)
		logrus.Infof("Scanning %d recommended images", len(recommendationsToScan))
		recommendationReport := image.ScanImages(recommendationsToScan, maxConcurrentScans, extraFlags, true)
		logrus.Infof("Dones scanning recommendations")
		recommendationsToKeep := getRecommendationImagesToKeep(images, *lastReport, recommendationsToScan)
		lastReport.Images = append(clusterImagesToKeep, recommendationsToKeep...)
		aggregated = append(allReports, recommendationReport...)
	}
	logrus.Infof("Done with all scans, minimizing report")
	minimizedReport := image.Minimize(aggregated, *lastReport)
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

func getNewestVersionsToScan(ctx context.Context, allReports []models.ImageReport, imagesToScan []models.Image) []models.Image {
	var imageWithVulns []models.ImageReport
	for _, img := range imagesToScan {
		imageSha := image.GetShaFromID(img.ID)
		for _, report := range allReports {
			reportSha := image.GetShaFromID(report.ID)
			if report.Name == img.Name && reportSha == imageSha {
				if len(report.Reports) > 0 {
					imageWithVulns = append(imageWithVulns, report)
				}
			}
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
		versionsChan <- newestVersions{
			err: fmt.Errorf("cannot find tag while getting newest version for image %q", img.Name),
		}
		return
	}
	repo := parts[0]
	tag := parts[1]
	if strings.Contains(strings.ToLower(img.Name), "@sha256:") {
		// Do not try to find newer versions when the tag is a sha256.
		repo = strings.Split(repo, "@")[0]
		logrus.Debugf("not getting newest versions for repo %q because the tag is a sha256: %q", repo, img.Name)
		versionsChan <- newestVersions{
			repo:     repo,
			versions: []string{},
		}
		return
	}
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

func getUnscannedImagesToScan(images []models.Image, lastReportImages []models.ImageDetailsWithRefs) []models.Image {
	alreadyAdded := map[string]bool{}
	imagesToScan := make([]models.Image, 0)
	for _, img := range images {
		imageSha := image.GetShaFromID(img.ID)
		found := false
		for _, report := range lastReportImages {
			reportSha := image.GetShaFromID(report.ID)
			if report.Name == img.Name && reportSha == imageSha {
				found = true
				break
			}
		}
		if !found && !alreadyAdded[imageSha] {
			imagesToScan = append(imagesToScan, img)
			alreadyAdded[imageSha] = true
		}
	}
	if len(imagesToScan) > numberToScan {
		imagesToScan = imagesToScan[:numberToScan]
	}
	return imagesToScan
}

func getClusterImagesToKeep(images []models.Image, lastReport models.MinimizedReport, imagesToScan []models.Image) []models.ImageDetailsWithRefs {
	imagesToKeep := make([]models.ImageDetailsWithRefs, 0)
	scanned := convertImagesToMap(imagesToScan)
	for _, report := range lastReport.Images {
		reportSha := image.GetShaFromID(report.ID)
		if !report.RecommendationOnly {
			for _, img := range images {
				imageSha := image.GetShaFromID(img.ID)
				if report.Name == img.Name && reportSha == imageSha && !scanned[imageSha] {
					imagesToKeep = append(imagesToKeep, report)
					break
				}
			}
		}
	}
	return imagesToKeep
}

func getImagesToRescan(images []models.Image, lastReport models.MinimizedReport, imagesToScan []models.Image) []models.Image {
	sort.Slice(lastReport.Images, func(a, b int) bool {
		return lastReport.Images[a].LastScan == nil || lastReport.Images[b].LastScan != nil && lastReport.Images[a].LastScan.Before(*lastReport.Images[b].LastScan)
	})
	for _, report := range lastReport.Images {
		reportSha := image.GetShaFromID(report.ID)
		if !report.RecommendationOnly {
			for _, img := range images {
				imageSha := image.GetShaFromID(img.ID)
				if report.Name == img.Name && reportSha == imageSha {
					if len(imagesToScan) < numberToScan {
						imagesToScan = append(imagesToScan, img)
						break
					} else {
						return imagesToScan
					}
				}
			}
		}
	}
	return imagesToScan
}

func getRecommendationImagesToKeep(images []models.Image, lastReport models.MinimizedReport, recommendationsToScan []models.Image) []models.ImageDetailsWithRefs {
	imagesToKeep := make([]models.ImageDetailsWithRefs, 0)
	sort.Slice(lastReport.Images, func(a, b int) bool {
		return lastReport.Images[a].LastScan == nil || lastReport.Images[b].LastScan != nil && lastReport.Images[a].LastScan.Before(*lastReport.Images[b].LastScan)
	})
	newRecommendations := convertImagesToMap(recommendationsToScan)
	clusterImagesMap := imagesRepositoryMap(images)
	for _, report := range lastReport.Images {
		reportSha := image.GetShaFromID(report.ID)
		// We must keep images recommendations for those still in the cluster but not scanned at this time
		if report.RecommendationOnly {
			parts := strings.Split(report.Name, ":")
			if len(parts) == 2 {
				key := image.GetRecommendationKey(parts[0], image.GetSpecificToken(parts[1]))
				// Add old recommendations only if we have the images they are for still running in the cluster
				if _, found := clusterImagesMap[key]; found {
					// Add old recommendations only if we have not scanned for new recommendations
					if _, found := newRecommendations[reportSha]; !found {
						imagesToKeep = append(imagesToKeep, report)
					}
				}
			}
		}
	}
	return imagesToKeep
}

func convertImagesToMap(list []models.Image) map[string]bool {
	m := map[string]bool{}
	for _, img := range list {
		sha := image.GetShaFromID(img.ID)
		m[sha] = true
	}
	return m
}

func imagesRepositoryMap(list []models.Image) map[string]bool {
	m := map[string]bool{}
	for _, img := range list {
		parts := strings.Split(img.Name, ":")
		if len(parts) == 2 {
			key := image.GetRecommendationKey(parts[0], image.GetSpecificToken(parts[1]))
			m[key] = true
		}
	}
	return m
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

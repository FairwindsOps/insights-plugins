package image

import (
	"sort"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
	v2 "github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models/v2"
)

func GetMatchingImages(baseImages []v2.ImageDetailsWithRefs, toMatch []models.Image, isRecommendation bool) []v2.ImageDetailsWithRefs {
	return getImages(baseImages, toMatch, isRecommendation, true)
}

func GetUnmatchingImages(baseImages []v2.ImageDetailsWithRefs, toMatch []models.Image, isRecommendation bool) []v2.ImageDetailsWithRefs {
	return getImages(baseImages, toMatch, isRecommendation, false)
}

func getImages(baseImages []v2.ImageDetailsWithRefs, toMatch []models.Image, isRecommendation bool, match bool) []v2.ImageDetailsWithRefs {
	filtered := make([]v2.ImageDetailsWithRefs, 0)
	isMatch := convertImagesToMap(toMatch)
	isRepoMatch := imagesRepositoryMap(toMatch)
	for _, im := range baseImages {
		if !isRecommendation {
			if im.RecommendationOnly || isMatch[im.GetUniqueID()] == match {
				filtered = append(filtered, im)
			}
		} else {
			// For recommendations, we match only on repo name, not on full image ID
			parts := strings.Split(im.Name, ":")
			key := GetRecommendationKey(parts[0], GetSpecificToken(parts[1]))
			if !im.RecommendationOnly || isRepoMatch[key] == match {
				filtered = append(filtered, im)
			}
		}
	}
	return filtered
}

func GetUnscannedImagesToScan(imagesInCluster []models.Image, lastReportImages []v2.ImageDetailsWithRefs, maxScans int) []models.Image {
	alreadyAdded := map[string]bool{}
	alreadyScanned := convertImagesWithRefsToMap(lastReportImages)
	imagesToScan := make([]models.Image, 0)
	for _, img := range imagesInCluster {
		if !alreadyScanned[img.GetUniqueID()] && !alreadyAdded[img.GetUniqueID()] {
			imagesToScan = append(imagesToScan, img)
			alreadyAdded[img.GetUniqueID()] = true
		}
	}
	if len(imagesToScan) > maxScans {
		imagesToScan = imagesToScan[:maxScans]
	}
	return imagesToScan
}

func GetImagesToRescan(images []models.Image, lastReport v2.MinimizedReport, imagesToScan []models.Image, maxScans int) []models.Image {
	sort.Slice(lastReport.Images, func(a, b int) bool {
		return lastReport.Images[a].LastScan == nil || lastReport.Images[b].LastScan != nil && lastReport.Images[a].LastScan.Before(*lastReport.Images[b].LastScan)
	})
	for _, report := range lastReport.Images {
		reportID := report.GetUniqueID()
		if !report.RecommendationOnly {
			for _, img := range images {
				imageID := img.GetUniqueID()
				if report.Name == img.Name && reportID == imageID {
					if len(imagesToScan) < maxScans {
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

func convertImagesToMap(list []models.Image) map[string]bool {
	m := map[string]bool{}
	for _, img := range list {
		m[img.GetUniqueID()] = true
	}
	return m
}

func convertImagesWithRefsToMap(list []v2.ImageDetailsWithRefs) map[string]bool {
	m := map[string]bool{}
	for _, img := range list {
		m[img.GetUniqueID()] = true
	}
	return m
}

func imagesRepositoryMap(list []models.Image) map[string]bool {
	m := map[string]bool{}
	for _, img := range list {
		parts := strings.Split(img.Name, ":")
		if len(parts) == 2 {
			key := GetRecommendationKey(parts[0], GetSpecificToken(parts[1]))
			m[key] = true
		}
	}
	return m
}

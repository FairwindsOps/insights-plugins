package image

import (
	"sort"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
)

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


func GetMatchingImages(baseImages []models.ImageDetailsWithRefs, toMatch []models.Image, isRecommendation bool) []models.ImageDetailsWithRefs {
  return getImages(baseImages, toMatch, isRecommendation, true)
}

func GetUnmatchingImages(baseImages []models.ImageDetailsWithRefs, toMatch []models.Image, isRecommendation bool) []models.ImageDetailsWithRefs {
  return getImages(baseImages, toMatch, isRecommendation, false)
}

func getImages(baseImages []models.ImageDetailsWithRefs, toMatch []models.Image, isRecommendation bool, match bool) []models.ImageDetailsWithRefs {
	filtered := make([]models.ImageDetailsWithRefs, 0)
	isMatch := convertImagesToMap(toMatch)
	isRepoMatch:= imagesRepositoryMap(toMatch)
	for _, im := range baseImages {
		if !isRecommendation {
			imageSha := GetShaFromID(im.ID)
			if im.RecommendationOnly || isMatch[imageSha] == match {
				filtered = append(filtered, im)
			}
		} else {
			// For recommendations, we match only on repo name, not on full image ID
			parts := strings.Split(im.Name, ":")
			key := GetRecommendationKey(parts[0], GetSpecificToken(parts[1]))
			if !im.RecommendationOnly || isRepoMatch[key] == match{
				filtered = append(filtered, im)
			}
		}
	}
	return filtered
}

func GetUnscannedImagesToScan(images []models.Image, lastReportImages []models.ImageDetailsWithRefs, maxScans int) []models.Image {
	alreadyAdded := map[string]bool{}
	imagesToScan := make([]models.Image, 0)
	for _, img := range images {
		imageSha := GetShaFromID(img.ID)
		found := false
		for _, report := range lastReportImages {
			reportSha := GetShaFromID(report.ID)
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
	if len(imagesToScan) > maxScans {
		imagesToScan = imagesToScan[:maxScans]
	}
	return imagesToScan
}

func GetImagesToRescan(images []models.Image, lastReport models.MinimizedReport, imagesToScan []models.Image, maxScans int) []models.Image {
	sort.Slice(lastReport.Images, func(a, b int) bool {
		return lastReport.Images[a].LastScan == nil || lastReport.Images[b].LastScan != nil && lastReport.Images[a].LastScan.Before(*lastReport.Images[b].LastScan)
	})
	for _, report := range lastReport.Images {
		reportSha := GetShaFromID(report.ID)
		if !report.RecommendationOnly {
			for _, img := range images {
				imageSha := GetShaFromID(img.ID)
				if report.Name == img.Name && reportSha == imageSha {
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

func GetRecommendationImagesToKeep(images []models.Image, lastReport models.MinimizedReport, recommendationsToScan []models.Image) []models.ImageDetailsWithRefs {
	imagesToKeep := make([]models.ImageDetailsWithRefs, 0)
	sort.Slice(lastReport.Images, func(a, b int) bool {
		return lastReport.Images[a].LastScan == nil || lastReport.Images[b].LastScan != nil && lastReport.Images[a].LastScan.Before(*lastReport.Images[b].LastScan)
	})
	newRecommendations := convertImagesToMap(recommendationsToScan)
	clusterImagesMap := imagesRepositoryMap(images)
	for _, report := range lastReport.Images {
		reportSha := GetShaFromID(report.ID)
		// We must keep images recommendations for those still in the cluster but not scanned at this time
		if report.RecommendationOnly {
			parts := strings.Split(report.Name, ":")
			if len(parts) == 2 {
				key := GetRecommendationKey(parts[0], GetSpecificToken(parts[1]))
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
		sha := GetShaFromID(img.ID)
		m[sha] = true
	}
	return m
}

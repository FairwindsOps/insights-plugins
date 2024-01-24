package image

import (
	"sort"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
)

func GetMatchingImages(baseImages []models.ImageDetailsWithRefs, toMatch []models.Image, isRecommendation bool) []models.ImageDetailsWithRefs {
	return getImages(baseImages, toMatch, isRecommendation, true)
}

func GetUnmatchingImages(baseImages []models.ImageDetailsWithRefs, toMatch []models.Image, isRecommendation bool) []models.ImageDetailsWithRefs {
	return getImages(baseImages, toMatch, isRecommendation, false)
}

func getImages(baseImages []models.ImageDetailsWithRefs, toMatch []models.Image, isRecommendation bool, match bool) []models.ImageDetailsWithRefs {
	filtered := make([]models.ImageDetailsWithRefs, 0)
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

func GetUnscannedImagesToScan(imagesInCluster []models.Image, lastReportImages []models.ImageDetailsWithRefs, maxScans int) []models.Image {
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

func GetImagesToReScan(images []models.Image, lastReport models.MinimizedReport, imagesToScan []models.Image, maxScans int) []models.Image {
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

func convertImagesWithRefsToMap(list []models.ImageDetailsWithRefs) map[string]bool {
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

func UpdateOwnersReferenceOnMatchingImages(baseImages []models.ImageDetailsWithRefs, clusterImages []models.Image) []models.ImageDetailsWithRefs {
	imageKeyToMap := map[string][]models.Resource{}
	for _, i := range clusterImages {
		imageKeyToMap[i.GetUniqueID()] = i.Owners
	}

	for i, img := range baseImages {
		if owners, ok := imageKeyToMap[img.GetUniqueID()]; ok {
			v2owners := []models.Resource{}
			for _, o := range owners {
				v2owners = append(v2owners, models.Resource{
					Name:      o.Name,
					Kind:      o.Kind,
					Namespace: o.Namespace,
					Container: o.Container,
				})
			}
			baseImages[i].Owners = v2owners
		}
	}
	return baseImages
}

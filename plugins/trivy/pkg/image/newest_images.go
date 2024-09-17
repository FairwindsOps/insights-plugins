package image

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	semver "github.com/Masterminds/semver/v3"

	"github.com/sirupsen/logrus"
)

var knownPreReleases = []string{
	"centos-7",
	"debian-8",
	"debian-9",
	"debian-10",
	"ol-7",
	"ubuntu",
	"amd64",
	"alpine",
	"bullseye",
}

const MaxNewestVersionsToScan = 1

// GetNewestVersions returns newest versions and newest version within same major version
func GetNewestVersions(ctx context.Context, repo, tag string) ([]string, error) {
	logrus.Infof("started retrieving newest versions for %s:%s", repo, tag)
	tags, err := fetchTags(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("error fetching tags for %s:%s: %w", repo, tag, err)
	}
	newest, err := filterAndSort(tags, tag)
	if err != nil {
		return nil, err
	}
	logrus.Infof("finished retrieving newest versions for %s:%s", repo, tag)
	if len(newest) <= MaxNewestVersionsToScan {
		return newest, nil
	}
	return newest[len(newest)-MaxNewestVersionsToScan:], nil
}

func fetchTags(ctx context.Context, imageRepoName string) ([]string, error) {
	repository, err := name.NewRepository(imageRepoName)
	if err != nil {
		return nil, err
	}

	// GET @ https://{quay.io|index.docker.io|k8s.gcr.io}/v2/fairwinds/insights-uploader/tags/list
	tags, err := remote.List(repository, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	logrus.Debugf("found tags %v for repo %s", tags, imageRepoName)
	return tags, nil
}

func filterAndSort(suggestedTags []string, curTagStr string) ([]string, error) {
	curVersion, err := semver.NewVersion(curTagStr)
	if err != nil {
		logrus.Debugf("could not parse current tag semver %s: %v", curTagStr, err)
		return []string{}, nil
	}

	constraint, err := semver.NewConstraint(">" + curTagStr)
	if err != nil {
		return nil, err
	}

	newest := []*semver.Version{}
	for _, tag := range suggestedTags {
		// bail-out in case of numeric short SHA to avoid semantically wrong semver
		// i.e.: 7875368 -> major: 7875368, minor: 0, patch: 0
		if isShortShaTag(tag) {
			continue
		}
		v, err := semver.NewVersion(tag)
		if err != nil {
			logrus.Debugf("could not parse tag semver %s: %v", tag, err)
			continue
		}

		cPre := curVersion.Prerelease()
		vPre := v.Prerelease()
		if cPre != "" && vPre != "" && cPre != vPre {
			logrus.Infof("pre-releases does not match: %s != %s", cPre, vPre)
			continue
		}

		check, errors := constraint.Validate(v)
		if check {
			newest = append(newest, v)
		} else {
			logrus.Debug(errors) // for debugging only
		}
	}
	Sort(newest)
	return Versions(newest).ToStringSlice(), nil
}

func isShortShaTag(tag string) bool {
	intTag, err := strconv.Atoi(tag)
	if err != nil {
		return false
	}
	return intTag > 1_000
}

func GetSpecificToken(tag string) string {
	for _, v := range knownPreReleases {
		if strings.Contains(tag, v) {
			return v
		}
	}
	return ""
}

func GetRecommendationKey(repoName, specific string) string {
	if specific == "" {
		return repoName
	}
	return repoName + "/" + specific
}

type NewestVersions struct {
	repo     string
	versions []string
	err      error
}

func GetNewestVersionsToScan(ctx context.Context, allReports []models.ImageReport, imagesToScan []models.Image) []models.Image {
	var imageWithVulns []models.ImageReport
	for _, img := range imagesToScan {
		imageSha := img.GetSha()
		for _, report := range allReports {
			reportSha := report.GetSha()
			if report.Name == img.Name && reportSha == imageSha {
				if len(report.Reports) > 0 {
					imageWithVulns = append(imageWithVulns, report)
				}
			}
		}
	}

	versionsChan := make(chan NewestVersions, len(imageWithVulns))
	for _, i := range imageWithVulns {
		logrus.Infof("launching go-routine to fetch newer versions for image %s", i.Name)
		go getNewestVersions(versionsChan, ctx, i)
	}

	newImagesToScan := []models.Image{}
	for i := 0; i < len(imageWithVulns); i++ {
		vc := <-versionsChan
		if vc.err != nil {
			logrus.Errorf("could not fetch newer versions for repo %s: %v", vc.repo, vc.err)
		}
		logrus.Infof("received newer versions for image %s - %v", vc.repo, vc.versions)
		for _, v := range vc.versions {
			newImagesToScan = append(newImagesToScan, models.Image{
				ID:                 fmt.Sprintf("%v:%v", vc.repo, v),
				Name:               fmt.Sprintf("%v:%v", vc.repo, v),
				PullRef:            fmt.Sprintf("%v:%v", vc.repo, v),
				RecommendationOnly: true,
			})
		}
	}
	return newImagesToScan
}

func getNewestVersions(versionsChan chan NewestVersions, ctx context.Context, img models.ImageReport) {
	parts := strings.Split(img.Name, ":")
	if len(parts) != 2 {
		versionsChan <- NewestVersions{
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
		versionsChan <- NewestVersions{
			repo:     repo,
			versions: []string{},
		}
		return
	}
	versions, err := GetNewestVersions(ctx, repo, tag)
	if err != nil {
		versionsChan <- NewestVersions{
			repo: repo,
			err:  err,
		}
		return
	}
	versionsChan <- NewestVersions{
		repo:     repo,
		versions: versions,
	}
}

package image

import (
	"context"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/samber/lo"

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
	logrus.Info("Started retrieving newest versions for ", repo, ":", tag)
	tags, err := fetchTags(ctx, repo)
	if err != nil {
		logrus.Error("Error fetching tags for ", repo, ":", tag, err)
		return nil, err

	}
	newest, err := filterAndSort(tags, tag)
	if err != nil {
		return nil, err
	}
	logrus.Info("Finished retrieving newest versions for ", repo, ":", tag)
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
	tags, err := remote.List(repository, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return tags, nil
}

func filterAndSort(suggestedTags []string, curTagStr string) ([]string, error) {
	curVersion, err := semver.NewVersion(curTagStr)
	if err != nil {
		logrus.Infof("could not parse current version %s: %v", curTagStr, err)
		return []string{}, nil
	}

	constraint, err := semver.NewConstraint(">" + curTagStr)
	if err != nil {
		return nil, err
	}

	newest := []*semver.Version{}
	for _, tag := range suggestedTags {
		v, err := semver.NewVersion(tag)
		if err != nil {
			logrus.Infof("could not parse tag %s: %v", tag, err)
			continue
		}

		cPre := curVersion.Prerelease()
		vPre := v.Prerelease()
		isKnownPreRelease := lo.Contains(knownPreReleases, cPre)
		if isKnownPreRelease && vPre != "" && cPre != vPre {
			logrus.Infof("prerelease does not match: %s != %s", cPre, vPre)
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

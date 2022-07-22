package image

import (
	"context"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	version "github.com/mcuadros/go-version"
	"github.com/sirupsen/logrus"
)

var specific = []string{
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
	newest := filterAndSort(tags, tag)
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
	logrus.Infof("Fetched %d tags for  %s", len(tags), imageRepoName)
	return tags, nil
}

func filterAndSort(tags []string, currentTag string) []string {
	newest := []string{}
	c := version.NewConstrainGroupFromString(">" + currentTag)
	currentTagSpecificToken := GetSpecificToken(currentTag)
	for _, targetTag := range tags {
		targetTagSpecificToken := GetSpecificToken(targetTag)
		if c.Match(targetTag) && currentTagSpecificToken == targetTagSpecificToken {
			newest = append(newest, targetTag)
		}
	}
	version.Sort(newest)
	return newest
}

func GetSpecificToken(tag string) string {
	for _, v := range specific {
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

package image

import (
	"context"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/genuinetools/reg/registry"
	"github.com/genuinetools/reg/repoutils"
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

// GetNewestVersions returns newest versions and newest version within same major version
func GetNewestVersions(ctx context.Context, repo, tag string) ([]string, error) {
	logrus.Info("Started retrieving newest versions for ", repo, ":", tag)
	tags, err := fetchTags(ctx, repo, tag)
	if err != nil {
		logrus.Error("Error fetching tags for ", repo, ":", tag, err)
		return nil, err

	}
	newest := filterAndSort(tags, tag)
	logrus.Info("Finished retrieving newest versions for ", repo, ":", tag)
	if len(newest) <= 1 {
		return newest, nil
	}
	return newest[len(newest)-1:], nil
}

func fetchTags(ctx context.Context, imageName, tag string) ([]string, error) {
	image, err := registry.ParseImage(imageName)
	if err != nil {
		return nil, err
	}
	r, err := createRegistryClient(ctx, image.Domain)
	if err != nil {
		return nil, err
	}
	tags, err := r.Tags(ctx, image.Path)
	if err != nil {
		return nil, err
	}
	return tags, nil
}

func createRegistryClient(ctx context.Context, domain string) (*registry.Registry, error) {
	auth := types.AuthConfig{}
	if domain == "docker.io" {
		auth.ServerAddress = repoutils.DefaultDockerRegistry
	}
	return registry.New(ctx, auth, registry.Opt{
		Domain:   domain,
		Insecure: false,
		Debug:    false,
		SkipPing: false,
		NonSSL:   false,
	})
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

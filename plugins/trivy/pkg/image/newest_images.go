package image

import (
	"context"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/genuinetools/reg/registry"
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
	if len(newest) <= 2 {
		return newest, nil
	}
	return newest[len(newest)-2:], nil
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
	return registry.New(ctx, types.AuthConfig{}, registry.Opt{
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
	filter := ""
	for _, v := range specific {
		if strings.Contains(currentTag, v) {
			filter = v
			break
		}
	}
	for _, tag := range tags {
		if c.Match(tag) && (filter == "" || strings.Contains(tag, filter)) {
			newest = append(newest, tag)
		}
	}
	version.Sort(newest)
	return newest
}
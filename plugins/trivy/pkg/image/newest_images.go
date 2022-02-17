package image

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/genuinetools/reg/registry"
	"github.com/genuinetools/reg/repoutils"
	version "github.com/mcuadros/go-version"
	"github.com/sirupsen/logrus"
)

var (
	insecure    bool
	forceNonSSL bool
	skipPing    bool

	timeout time.Duration

	authURL  string
	username string
	password string

	debug bool
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
func GetNewestVersions(repo, tag string) ([]string, error) {
	logrus.Info("Started retrieving newest versions for %v:%v", repo, tag)
	tags, err := fetchTags(repo, tag)
	if err != nil {
		logrus.Error("Error fetching tags for for %v:%v: %v", repo, tag, err)
		return nil, err

	}
	newest := filterAndSort(tags, tag)
	logrus.Info("Finished retrieving newest versions for %v:%v", repo, tag)
	if len(newest) < 2 {
		return newest, nil
	}
	return newest, nil
}

func fetchTags(imageName, tag string) ([]string, error) {
	image, err := registry.ParseImage(imageName)
	if err != nil {
		return nil, err
	}
	r, err := createRegistryClient(context.TODO(), image.Domain)
	if err != nil {
		return nil, err
	}
	tags, err := r.Tags(context.TODO(), image.Path)
	if err != nil {
		return nil, err
	}
	return tags, nil
}

func createRegistryClient(ctx context.Context, domain string) (*registry.Registry, error) {
	authDomain := authURL
	if authDomain == "" {
		authDomain = domain
	}
	auth, err := repoutils.GetAuthConfig(username, password, authDomain)
	if err != nil {
		return nil, err
	}
	if !forceNonSSL && strings.HasPrefix(auth.ServerAddress, "http:") {
		return nil, fmt.Errorf("attempted to use insecure protocol! Use force-non-ssl option to force")
	}
	return registry.New(ctx, auth, registry.Opt{
		Domain:   domain,
		Insecure: insecure,
		Debug:    debug,
		SkipPing: skipPing,
		NonSSL:   forceNonSSL,
		Timeout:  timeout,
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

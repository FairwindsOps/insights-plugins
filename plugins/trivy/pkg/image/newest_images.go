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

func GetNewestVersions(repo, tag string) ([]string, []string, error) {
	logrus.Info("Started retrieving newest versions for %v:%v", repo, tag)
	tags, err := fetchTags(repo, tag)
	if err != nil {
		logrus.Error("Error fetching tags for for %v:%v: %v", repo, tag, error)
		return nil, nil, err

	}
	newest, sameMajor := filterAndSort(tags, tag)
	logrus.Info("Finished retrieving newest versions for %v:%v", repo, tag)
	return newest, sameMajor, nil
}

func fetchTags(imageName, tag string) ([]string, error) {
	image, err := registry.ParseImage(imageName)
	if err != nil {
		return nil, err
	}
	// Create the registry client.
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
	// Use the auth-url domain if provided.
	authDomain := authURL
	if authDomain == "" {
		authDomain = domain
	}
	auth, err := repoutils.GetAuthConfig(username, password, authDomain)
	if err != nil {
		return nil, err
	}
	// Prevent non-ssl unless explicitly forced
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

func filterAndSort(tags []string, currentTag string) ([]string, []string) {
	newest := []string{}
	sameMajor := []string{}
	major := strings.Split(currentTag, ".")[0] + "."
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
		if c.Match(tag) && strings.HasPrefix(tag, major) {
			sameMajor = append(sameMajor, tag)
		}
	}
	version.Sort(newest)
	return newest, sameMajor
}

package ci

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
	trivymodels "github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"

	"github.com/stretchr/testify/assert"
)

func TestWalkImages(t *testing.T) {
	err := walkImages("testdata/images/nginx1232alpine.tar", func(filename, sha string, tags []string) {
		assert.Equal(t, "nginx1232alpine.tar", filename)
		assert.Equal(t, "sha256:2c8fe00b7b5c2a860377214dbf1c6b6bf7cee4d8d31df9672680a9a0186f2c22", sha)
		assert.Equal(t, []string{"nginx:1.23.2-alpine"}, tags)
	})
	assert.NoError(t, err)
}

func TestClearString(t *testing.T) {
	assert.Equal(t, "postgres_15_1_bullseye", clearString("postgres:15.1-bullseye"))
	assert.Equal(t, "nginx_1_23_2_alpine", clearString("nginx:1.23.2-alpine"))
	assert.Equal(t, "fedora_httpd_version1_0", clearString("fedora/httpd:version1.0"))
	assert.Equal(t, "myregistryhost_5000_fedora_httpd_version1_0", clearString("myregistryhost:5000/fedora/httpd:version1.0"))
}

func TestDownloadImageViaSkopeo(t *testing.T) {
	noopReturnArgsCmdExecutor := func(cmd *exec.Cmd, message string) (string, error) {
		return fmt.Sprint(cmd.Args), nil
	}

	// #1 - no registry credential
	cmd, err := downloadImageViaSkopeo(noopReturnArgsCmdExecutor, "./", "postgres:15.1-bullseye", nil)
	assert.NoError(t, err)
	assert.Equal(t, "[skopeo copy docker://postgres:15.1-bullseye docker-archive:./postgres_15_1_bullseye]", cmd)

	// #2 - with registry credential
	rc := models.RegistryCredential{Domain: "docker.io", Username: "my-username", Password: "my-password"}
	cmd, err = downloadImageViaSkopeo(noopReturnArgsCmdExecutor, "./", "postgres:15.1-bullseye", &rc)
	assert.NoError(t, err)
	assert.Equal(t, "[skopeo copy --src-creds my-username:my-password docker://postgres:15.1-bullseye docker-archive:./postgres_15_1_bullseye]", cmd)

	// #3 - with registry credential using token
	rc = models.RegistryCredential{Domain: "docker.io", Username: "<token>", Password: "my-bearer-token"}
	cmd, err = downloadImageViaSkopeo(noopReturnArgsCmdExecutor, "./", "postgres:15.1-bullseye", &rc)
	assert.NoError(t, err)
	assert.Equal(t, "[skopeo copy --src-registry-token my-bearer-token docker://postgres:15.1-bullseye docker-archive:./postgres_15_1_bullseye]", cmd)

	// #4 - SKOPEO_ARGS args test
	os.Setenv("SKOPEO_ARGS", "random args")

	rc = models.RegistryCredential{Domain: "docker.io", Username: "my-username", Password: "my-password"}
	cmd, err = downloadImageViaSkopeo(noopReturnArgsCmdExecutor, "./", "postgres:15.1-bullseye", &rc)
	assert.NoError(t, err)
	assert.Equal(t, "[skopeo copy --src-creds my-username:my-password random args docker://postgres:15.1-bullseye docker-archive:./postgres_15_1_bullseye]", cmd)
}

func TestDownloadMissingImages(t *testing.T) {
	mockDownloaderFn := func(cmdExecutor cmdExecutor, folderPath, imageName string, rc *models.RegistryCredential) (string, error) {
		return "", nil // noop
	}
	rc := models.RegistryCredentials{}

	dockerImages := []trivymodels.DockerImage{{Name: "postgres:15.1-bullseye", PullRef: "postgres_15_1_bullseye"}, {Name: "nginx:1.23.2-alpine", PullRef: "nginx_1_23_2_alpine"}}
	manifestImages := []trivymodels.Image{
		{
			Name:   "postgres:15.1-bullseye",
			Owners: []trivymodels.Resource{{Name: "postgres", Kind: "Deployment", Namespace: "default", Container: "postgres"}},
		},
		{
			Name:   "nginx:1.23.2-alpine",
			Owners: []trivymodels.Resource{{Name: "nginx", Kind: "Deployment", Namespace: "default", Container: "nginx"}},
		},
		{
			Name:   "alpine:1.24.2",
			Owners: []trivymodels.Resource{{Name: "nginx", Kind: "Deployment", Namespace: "default", Container: "nginx"}},
		},
	}

	refToImageName, dockerImages, manifestImages, err := downloadMissingImages("_/images", mockDownloaderFn, dockerImages, manifestImages, rc)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"postgres_15_1_bullseye": "postgres:15.1-bullseye",
		"nginx_1_23_2_alpine":    "nginx:1.23.2-alpine",
		"alpine_1_24_2":          "alpine:1.24.2",
	}, refToImageName)

	assert.Len(t, dockerImages, 2)
	assert.Len(t, manifestImages, 3)

	for _, mi := range manifestImages {
		assert.NotEmpty(t, mi.PullRef)
	}
}

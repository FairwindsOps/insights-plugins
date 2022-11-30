package ci

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"

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
	assert.Equal(t, "nginx1232alpine", clearString("nginx:1.23.2-alpine"))
	assert.Equal(t, "fedorahttpdversion10", clearString("fedora/httpd:version1.0"))
	assert.Equal(t, "myregistryhost5000fedorahttpdversion10", clearString("myregistryhost:5000/fedora/httpd:version1.0"))
}

func TestDownloadImageViaSkopeo(t *testing.T) {
	noopReturnArgsCmdExecutor := func(cmd *exec.Cmd, message string) (string, error) {
		return fmt.Sprint(cmd.Args), nil
	}

	// #1 - no registry credential
	cmd, err := downloadImageViaSkopeo(noopReturnArgsCmdExecutor, "./", "postgres:15.1-bullseye", nil)
	assert.NoError(t, err)
	assert.Equal(t, "[skopeo copy docker://postgres:15.1-bullseye docker-archive:./postgres151bullseye]", cmd)

	// #2 - with registry credential
	rc := models.RegistryCredential{Domain: "docker.io", Username: "my-username", Password: "my-password"}
	cmd, err = downloadImageViaSkopeo(noopReturnArgsCmdExecutor, "./", "postgres:15.1-bullseye", &rc)
	assert.NoError(t, err)
	assert.Equal(t, "[skopeo copy --src-creds my-username:my-password docker://postgres:15.1-bullseye docker-archive:./postgres151bullseye]", cmd)

	// #3 - with registry credential using token
	rc = models.RegistryCredential{Domain: "docker.io", Username: "<token>", Password: "my-bearer-token"}
	cmd, err = downloadImageViaSkopeo(noopReturnArgsCmdExecutor, "./", "postgres:15.1-bullseye", &rc)
	assert.NoError(t, err)
	assert.Equal(t, "[skopeo copy --src-registry-token my-bearer-token docker://postgres:15.1-bullseye docker-archive:./postgres151bullseye]", cmd)

	// #4 - SKOPEO_ARGS args test
	os.Setenv("SKOPEO_ARGS", "random args")

	rc = models.RegistryCredential{Domain: "docker.io", Username: "my-username", Password: "my-password"}
	cmd, err = downloadImageViaSkopeo(noopReturnArgsCmdExecutor, "./", "postgres:15.1-bullseye", &rc)
	assert.NoError(t, err)
	assert.Equal(t, "[skopeo copy --src-creds my-username:my-password random args docker://postgres:15.1-bullseye docker-archive:./postgres151bullseye]", cmd)
}

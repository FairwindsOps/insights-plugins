package ci

import (
	"testing"

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

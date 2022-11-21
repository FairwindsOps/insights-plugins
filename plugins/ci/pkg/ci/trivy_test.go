package ci

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWalkImages(t *testing.T) {
	err := walkImages("testdata/images/postgres151bullseye.tar", func(filename, sha string, tags []string) {
		assert.Equal(t, "postgres151bullseye.tar", filename)
		assert.Equal(t, "sha256:5918a4f7e04aed7ac69d2b03b9b91345556db38709f9d6354056e3fdd9a8c02f", sha)
		assert.Equal(t, []string{"postgres:15.1-bullseye"}, tags)
	})
	assert.NoError(t, err)
}

func TestClearString(t *testing.T) {
	assert.Equal(t, "postgres151bullseye", clearString("postgres:15.1-bullseye"))
	assert.Equal(t, "fedorahttpdversion10", clearString("fedora/httpd:version1.0"))
	assert.Equal(t, "myregistryhost5000fedorahttpdversion10", clearString("myregistryhost:5000/fedora/httpd:version1.0"))
}

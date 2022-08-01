package image

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterAndSort(t *testing.T) {
	tags := []string{"6b6d653", "v0.0.14", "v0.0.22", "v0.0.16", "v0.0.16-alpine", "v0.0.14-beta2"}
	newestTags, err := filterAndSort(tags, "v0.0.14-beta1")
	assert.NoError(t, err)
	assert.Equal(t, []string{"v0.0.14", "v0.0.16-alpine", "v0.0.16", "v0.0.22"}, newestTags)
}

func TestFilterAndSor1(t *testing.T) {
	tags := []string{"6b6d653", "v0.0.14", "v0.0.14-beta1", "v0.0.22", "v0.0.22-alpine", "v0.0.16"}
	newestTags, err := filterAndSort(tags, "v0.0.14")
	assert.NoError(t, err)
	assert.Equal(t, []string{"v0.0.16", "v0.0.22"}, newestTags)
}

func TestFilterAndSort2(t *testing.T) {
	tags := []string{"6b6d653", "v0.0.14", "v0.0.14-beta1", "v0.0.22", "v0.0.16"}
	newestTags, err := filterAndSort(tags, "6b6d653")
	assert.NoError(t, err)
	assert.Equal(t, []string{}, newestTags)
}

func TestFilterAndSort3(t *testing.T) {
	tags := []string{"0.1.2-ubuntu", "0.1.3-alpine", "0.1.1-alpine", "0.1.1-beta1"}
	newestTags, err := filterAndSort(tags, "0.1.0-alpine")
	assert.NoError(t, err)
	assert.Equal(t, []string{"0.1.1-alpine", "0.1.3-alpine"}, newestTags)
}

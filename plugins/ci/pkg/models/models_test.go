package models

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetEmptyDefaults(t *testing.T) {
	cfg := Configuration{}

	cfg.SetDefaults()

	assert.Equal(t, "master", cfg.Options.BaseBranch)
	assert.Equal(t, "https://insights.fairwinds.com", cfg.Options.Hostname)
	assert.Empty(t, cfg.Options.Organization)
	assert.Empty(t, cfg.Options.RepositoryName)
	assert.Empty(t, cfg.Options.CIRunner)
}

func TestSetEnvironmentDefaults(t *testing.T) {
	// set env vars
	os.Setenv("BASE_BRANCH", "main")
	os.Setenv("ORG_NAME", "acme-co")
	os.Setenv("REPOSITORY_NAME", "acme-co/repo1")
	os.Setenv("HOSTNAME", "http://insights.test.fairwinds.com")
	os.Setenv("CI_RUNNER", "github-actions")

	cfg := Configuration{}
	cfg.SetDefaults()

	assert.Equal(t, "main", cfg.Options.BaseBranch)
	assert.Equal(t, "http://insights.test.fairwinds.com", cfg.Options.Hostname)
	assert.Equal(t, "acme-co", cfg.Options.Organization)
	assert.Equal(t, "acme-co/repo1", cfg.Options.RepositoryName)
	assert.Equal(t, GithubActions, cfg.Options.CIRunner)
}

package models

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// note: do not set HOSTNAME to not override CIRCLECI assigned variable

func TestSetEmptyDefaults(t *testing.T) {
	cfg := Configuration{}

	err := cfg.SetDefaults()
	assert.NoError(t, err)

	assert.Equal(t, "master", cfg.Options.BaseBranch)
	assert.Empty(t, cfg.Options.Organization)
	assert.Empty(t, cfg.Options.RepositoryName)
	assert.Empty(t, cfg.Options.CIRunner)
}

func TestSetEnvironmentDefaults(t *testing.T) {
	// set env vars
	os.Setenv("BASE_BRANCH", "main")
	os.Setenv("ORG_NAME", "acme-co")
	os.Setenv("REPOSITORY_NAME", "acme-co/repo1")
	os.Setenv("CI_RUNNER", "github-actions")

	cfg := Configuration{}
	err := cfg.SetDefaults()
	assert.NoError(t, err)

	assert.Equal(t, "main", cfg.Options.BaseBranch)
	assert.Equal(t, "acme-co", cfg.Options.Organization)
	assert.Equal(t, "acme-co/repo1", cfg.Options.RepositoryName)
	assert.Equal(t, GithubActions, cfg.Options.CIRunner)
}

func TestConfigSetDefaultsRegistryCredentials(t *testing.T) {
	os.Setenv("REGISTRY_CREDENTIALS", `[{"domain": "docker.io", "username": "my-user", "password": "p@ssw0rdz"}]`)

	cfg := Configuration{}
	err := cfg.SetDefaults()
	assert.NoError(t, err)

	assert.Len(t, cfg.Options.RegistryCredentials, 1)
	expectedRegistryCredentials := RegistryCredentials{{Domain: "docker.io", Username: "my-user", Password: "p@ssw0rdz"}}
	assert.Equal(t, expectedRegistryCredentials, cfg.Options.RegistryCredentials)

	// duplicated domains

	os.Setenv("REGISTRY_CREDENTIALS", `[{"domain": "docker.io", "username": "my-user-0", "password": "p@ssw0rdz0"}, {"domain": "docker.io", "username": "my-user-1", "password": "p@ssw0rdz1"}]`)

	cfg = Configuration{}
	err = cfg.SetDefaults()
	assert.Error(t, err, "should error on duplicated domains")
}

func TestRegistryCredentialsString(t *testing.T) {
	rc := RegistryCredential{
		Domain:   "docker.io",
		Username: "username",
		Password: "password",
	}
	assert.Equal(t, "domain: docker.io, username: username, Password: ********", fmt.Sprint(rc), "password should be hidden on print")
}

func TestFindCredentialForImage(t *testing.T) {
	// #1
	registryCredentials := RegistryCredentials{}
	rc := registryCredentials.FindCredentialForImage("postgres:15.1-bullseye")
	assert.Nil(t, rc)

	// #2
	registryCredentials = RegistryCredentials{
		{
			Domain:   "quay.io",
			Username: "username",
			Password: "password",
		},
	}

	rc = registryCredentials.FindCredentialForImage("postgres:15.1-bullseye")
	assert.Nil(t, rc)

	// #3
	registryCredentials = RegistryCredentials{
		{
			Domain:   "docker.io",
			Username: "username",
			Password: "password",
		},
		{
			Domain:   "quay.io",
			Username: "username",
			Password: "password",
		},
	}

	rc = registryCredentials.FindCredentialForImage("postgres:15.1-bullseye")
	assert.NotNil(t, rc)
	assert.Equal(t, "docker.io", rc.Domain)

	rc = registryCredentials.FindCredentialForImage("username/postgres:15.1-bullseye")
	assert.NotNil(t, rc)
	assert.Equal(t, "docker.io", rc.Domain)

	rc = registryCredentials.FindCredentialForImage("quay.io/osbuild/postgres:13-alpine-202211021552")
	assert.NotNil(t, rc)
	assert.Equal(t, "quay.io", rc.Domain)
}

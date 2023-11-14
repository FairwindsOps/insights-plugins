package ci

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestExtractRepoNameFromOrigin(t *testing.T) {
	assert.Equal(t, "FairwindsOps/insights-plugins", extractRepoNameFromOrigin("git@github.com:FairwindsOps/insights-plugins"))
	assert.Equal(t, "FairwindsOps/insights-plugins", extractRepoNameFromOrigin("git@github.com:FairwindsOps/insights-plugins.git"))
	assert.Equal(t, "FairwindsOps/insights-plugins", extractRepoNameFromOrigin("git@github.com:FairwindsOps/insights-plugins.git\n"))
	assert.Equal(t, "", extractRepoNameFromOrigin(""))
}

func TestGetGitInfo(t *testing.T) {
	r, err := getGitInfo(successfulStubExecutor, models.GithubActions, ".", "fairwinds/insights-plugins", "main")
	assert.NoError(t, err)
	assert.Equal(t, &gitInfo{origin: "origin-url", branch: "branch-name", masterHash: "master-hash-1", currentHash: "current-hash", commitMessage: "commit-message", repoName: "fairwinds/insights-plugins"}, r)

	r, err = getGitInfo(successfulStubExecutorOriginBranch, models.GithubActions, ".", "fairwinds/insights-plugins", "main")
	assert.NoError(t, err)
	assert.Equal(t, &gitInfo{origin: "origin-url", branch: "branch-name", masterHash: "master-hash-2", currentHash: "current-hash", commitMessage: "commit-message", repoName: "fairwinds/insights-plugins"}, r)

	r, err = getGitInfo(successfulStubExecutorRemotesOriginBranch, models.GithubActions, ".", "fairwinds/insights-plugins", "main")
	assert.NoError(t, err)
	assert.Equal(t, &gitInfo{origin: "origin-url", branch: "branch-name", masterHash: "master-hash-3", currentHash: "current-hash", commitMessage: "commit-message", repoName: "fairwinds/insights-plugins"}, r)

	r, err = getGitInfo(errorOnOptionalStubExecutor, models.GithubActions, ".", "fairwinds/insights-plugins", "main")
	assert.NoError(t, err)
	assert.Equal(t, &gitInfo{origin: "", branch: "", masterHash: "", currentHash: "current-hash", commitMessage: "", repoName: "fairwinds/insights-plugins"}, r)

	r, err = getGitInfo(errorOnRequiredStubExecutor, models.GithubActions, ".", "fairwinds/insights-plugins", "main")
	assert.Error(t, err)
	assert.Nil(t, r)
}

var successfulStubExecutor = func(dir string, cmd *exec.Cmd, message string) (string, error) {
	switch fmt.Sprintf("%v", cmd.Args) {
	case "[git config --global --add safe.directory /insights]": // required
		return "OK", nil
	case "[git rev-parse HEAD]": // required
		return "current-hash", nil
	case "[git merge-base HEAD main]":
		return "master-hash-1", nil
	case "[git merge-base HEAD origin/main]":
		return "master-hash-2", nil
	case "[git merge-base HEAD remotes/origin/main]":
		return "master-hash-3", nil
	case "[git log --pretty=format:%s -1]":
		return "commit-message", nil
	case "[git rev-parse --abbrev-ref HEAD]":
		return "branch-name", nil
	case "[git remote get-url origin]":
		return "origin-url", nil
	}
	return "", errors.New(fmt.Sprintf("command %v not mapped", cmd.Args))
}

var successfulStubExecutorOriginBranch = func(dir string, cmd *exec.Cmd, message string) (string, error) {
	switch fmt.Sprintf("%v", cmd.Args) {
	case "[git config --global --add safe.directory /insights]": // required
		return "OK", nil
	case "[git rev-parse HEAD]": // required
		return "current-hash", nil
	case "[git merge-base HEAD main]":
		return "", errors.New("could not fetch master-hash")
	case "[git merge-base HEAD origin/main]":
		return "master-hash-2", nil
	case "[git log --pretty=format:%s -1]":
		return "commit-message", nil
	case "[git rev-parse --abbrev-ref HEAD]":
		return "branch-name", nil
	case "[git remote get-url origin]":
		return "origin-url", nil
	}
	return "", errors.New(fmt.Sprintf("command %v not mapped", cmd.Args))
}

var successfulStubExecutorRemotesOriginBranch = func(dir string, cmd *exec.Cmd, message string) (string, error) {
	switch fmt.Sprintf("%v", cmd.Args) {
	case "[git config --global --add safe.directory /insights]": // required
		return "OK", nil
	case "[git rev-parse HEAD]": // required
		return "current-hash", nil
	case "[git merge-base HEAD main]":
		return "", errors.New("could not fetch master-hash-1")
	case "[git merge-base HEAD origin/main]":
		return "", errors.New("could not fetch master-hash-2")
	case "[git merge-base HEAD remotes/origin/main]":
		return "master-hash-3", nil
	case "[git log --pretty=format:%s -1]":
		return "commit-message", nil
	case "[git rev-parse --abbrev-ref HEAD]":
		return "branch-name", nil
	case "[git remote get-url origin]":
		return "origin-url", nil
	}
	return "", errors.New(fmt.Sprintf("command %v not mapped", cmd.Args))
}

var errorOnOptionalStubExecutor = func(dir string, cmd *exec.Cmd, message string) (string, error) {
	switch fmt.Sprintf("%v", cmd.Args) {
	case "[git config --global --add safe.directory /insights]": // required
		return "OK", nil
	case "[git rev-parse HEAD]": // required
		return "current-hash", nil
	case "[git merge-base HEAD main]":
		return "", errors.New("could not fetch master-hash-1")
	case "[git merge-base HEAD origin/main]":
		return "", errors.New("could not fetch master-hash-2")
	case "[git merge-base HEAD remotes/origin/main]":
		return "", errors.New("could not fetch master-hash-3")
	case "[git log --pretty=format:%s -1]":
		return "", errors.New("could not fetch commit-message")
	case "[git rev-parse --abbrev-ref HEAD]":
		return "", errors.New("could not fetch branch-name")
	case "[git remote get-url origin]":
		return "", errors.New("could not fetch origin-url")
	}
	return "", errors.New(fmt.Sprintf("command %v not mapped", cmd.Args))
}

var errorOnRequiredStubExecutor = func(dir string, cmd *exec.Cmd, message string) (string, error) {
	switch fmt.Sprintf("%v", cmd.Args) {
	case "[git config --global --add safe.directory /insights]": // required
		return "OK", nil
	case "[git rev-parse HEAD]": // required
		return "", errors.New("could not fetch current-hash")
	case "[git merge-base HEAD main]":
		return "", errors.New("could not fetch master-hash-1")
	case "[git merge-base HEAD origin/main]":
		return "", errors.New("could not fetch master-hash-2")
	case "[git merge-base HEAD remotes/origin/main]":
		return "", errors.New("could not fetch master-hash-3")
	case "[git log --pretty=format:%s -1]":
		return "", errors.New("could not fetch commit-message")
	case "[git rev-parse --abbrev-ref HEAD]":
		return "", errors.New("could not fetch branch-name")
	case "[git remote get-url origin]":
		return "", errors.New("could not fetch origin-url")
	}
	return "", errors.New(fmt.Sprintf("command %v not mapped", cmd.Args))
}

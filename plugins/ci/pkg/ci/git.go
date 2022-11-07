package ci

import (
	"os"
	"os/exec"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/util"
	"github.com/sirupsen/logrus"
)

type gitInfo struct {
	origin        string
	branch        string
	masterHash    string
	currentHash   string
	commitMessage string
	repoName      string
}

// cmdExecutor was extracted to be able to test this function - as the main implementation execute real commands on the given path
type cmdExecutor func(dir string, cmd *exec.Cmd, message string) (string, error)

func getGitInfo(cmdExecutor cmdExecutor, baseRepoPath, repoName, baseBranch string) (*gitInfo, error) {
	var err error
	_, err = cmdExecutor(baseRepoPath, exec.Command("git", "config", "--global", "--add", "safe.directory", "/insights"), "marking directory as safe")
	if err != nil {
		logrus.Errorf("Unable to mark directory %s as safe: %v", baseRepoPath, err)
		return nil, err
	}

	currentHash := os.Getenv("CURRENT_HASH")
	if currentHash == "" {
		currentHash, err = cmdExecutor(baseRepoPath, exec.Command("git", "rev-parse", "HEAD"), "getting current hash")
		if err != nil {
			logrus.Error("Unable to get GIT hash")
			return nil, err
		}
	}
	logrus.Infof("Current hash: %s", currentHash)

	masterHash := os.Getenv("MASTER_HASH")
	if masterHash == "" {
		masterHash, err = cmdExecutor(baseRepoPath, exec.Command("git", "merge-base", "HEAD", baseBranch), "getting master hash")
		if err != nil {
			logrus.Warnf("Unable to get GIT merge-base: %v", err)
		}
	}
	logrus.Infof("Master hash: %s", masterHash)

	commitMessage := os.Getenv("COMMIT_MESSAGE")
	if commitMessage == "" {
		commitMessage, err = cmdExecutor(baseRepoPath, exec.Command("git", "log", "--pretty=format:%s", "-1"), "getting commit message")
		if err != nil {
			logrus.Warnf("Unable to get GIT commit message: %v", err)
		}
	}
	if len(commitMessage) > 100 {
		commitMessage = commitMessage[:100] // Limit to 100 chars, double the length of github recommended length
	}
	logrus.Infof("Commit message: %s", commitMessage)

	branch := os.Getenv("BRANCH_NAME")
	if branch == "" {
		branch, err = cmdExecutor(baseRepoPath, exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD"), "getting branch name")
		if err != nil {
			logrus.Warnf("Unable to get GIT branch name: %v", err)
		}
	}
	logrus.Infof("Branch: %s", branch)

	origin := os.Getenv("ORIGIN_URL")
	if origin == "" {
		origin, err = cmdExecutor(baseRepoPath, exec.Command("git", "remote", "get-url", "origin"), "getting origin url")
		if err != nil {
			logrus.Warnf("Unable to get GIT origin: %v", err)
		}
	}
	logrus.Infof("Origin: %s", util.RemoveToken(origin))

	if repoName == "" {
		logrus.Infof("No repositoryName set, defaulting to origin.")
		repoName = extractRepoNameFromOrigin(origin)
	}
	logrus.Infof("Repo Name: %s", repoName)

	return &gitInfo{
		masterHash:    strings.TrimSuffix(masterHash, "\n"),
		currentHash:   strings.TrimSuffix(currentHash, "\n"),
		commitMessage: strings.TrimSuffix(commitMessage, "\n"),
		branch:        strings.TrimSuffix(branch, "\n"),
		origin:        strings.TrimSuffix(origin, "\n"),
		repoName:      strings.TrimSuffix(repoName, "\n"),
	}, nil
}

func extractRepoNameFromOrigin(origin string) string {
	var repoName = origin
	if strings.Contains(origin, "@") { // git@github.com URLs are allowed
		repoNameSplit := strings.Split(repoName, "@")
		// Take the substring after the last @ to avoid any tokens in an HTTPS URL
		repoName = repoNameSplit[len(repoNameSplit)-1]
	} else if strings.Contains(repoName, "//") {
		repoNameSplit := strings.Split(repoName, "//")
		repoName = repoNameSplit[len(repoNameSplit)-1]
	}
	// Remove "******.com:" prefix and ".git" suffix to get clean $org/$repo structure
	if strings.Contains(repoName, ":") {
		repoNameSplit := strings.Split(repoName, ":")
		repoName = repoNameSplit[len(repoNameSplit)-1]
	}
	return strings.TrimSuffix(repoName, ".git")
}

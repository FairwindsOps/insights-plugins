package ci

import (
	"os"
	"os/exec"
	"strings"

	"github.com/fairwindsops/insights-plugins/ci/pkg/commands"
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

func getGitInfo(baseRepoPath, repoName, baseBranch string) (*gitInfo, error) {
	var err error
	masterHash := os.Getenv("MASTER_HASH")
	if masterHash == "" {
		masterHash, err = commands.ExecInDir(baseRepoPath, exec.Command("git", "merge-base", "HEAD", baseBranch), "getting master hash")
		if err != nil {
			logrus.Error("Unable to get GIT merge-base")
			return nil, err
		}
	}

	currentHash := os.Getenv("CURRENT_HASH")
	if currentHash == "" {
		currentHash, err = commands.ExecInDir(baseRepoPath, exec.Command("git", "rev-parse", "HEAD"), "getting current hash")
		if err != nil {
			logrus.Error("Unable to get GIT Hash")
			return nil, err
		}
	}

	commitMessage := os.Getenv("COMMIT_MESSAGE")
	if commitMessage == "" {
		commitMessage, err = commands.ExecInDir(baseRepoPath, exec.Command("git", "log", "--pretty=format:%s", "-1"), "getting commit message")
		if err != nil {
			logrus.Error("Unable to get GIT Commit message")
			return nil, err
		}
	}
	if len(commitMessage) > 100 {
		commitMessage = commitMessage[:100] // Limit to 100 chars, double the length of github recommended length
	}
	branch := os.Getenv("BRANCH_NAME")
	if branch == "" {
		branch, err = commands.ExecInDir(baseRepoPath, exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD"), "getting branch name")
		if err != nil {
			logrus.Error("Unable to get GIT Branch Name")
			return nil, err
		}
	}
	origin := os.Getenv("ORIGIN_URL")
	if origin == "" {
		origin, err = commands.ExecInDir(baseRepoPath, exec.Command("git", "remote", "get-url", "origin"), "getting origin url")
		if err != nil {
			logrus.Error("Unable to get GIT Origin")
			return nil, err
		}
	}

	if repoName == "" {
		repoName = origin
		if strings.Contains(repoName, "@") { // git@github.com URLs are allowed
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
		repoName = strings.TrimSuffix(repoName, ".git")
	}
	return &gitInfo{
		masterHash:    strings.TrimSuffix(masterHash, "\n"),
		currentHash:   strings.TrimSuffix(currentHash, "\n"),
		commitMessage: strings.TrimSuffix(commitMessage, "\n"),
		branch:        strings.TrimSuffix(branch, "\n"),
		origin:        strings.TrimSuffix(origin, "\n"),
		repoName:      strings.TrimSuffix(repoName, "\n"),
	}, nil
}
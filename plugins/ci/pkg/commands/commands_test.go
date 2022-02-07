package commands

import (
	"os/exec"
	"testing"
)

// RunCommand runs a command and prints errors to Stderr
func TestFilterAccessTokenFromLogging(t *testing.T) {
	ExecInDir("", exec.Command("https://x-access-token:secrete-string@github.com/fairwindsops/repo1"), "cmd-with-access-token")
}

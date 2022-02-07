package commands

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

// RunCommand runs a command and prints errors to Stderr
func TestFilterAccessTokenFromLogging(t *testing.T) {
	_, err := ExecInDir("", exec.Command("https://x-access-token:secrete-string@github.com/fairwindsops/repo1"), "cmd-with-access-token")
	assert.NotContains(t, err.Error(), "secrete-string")
}

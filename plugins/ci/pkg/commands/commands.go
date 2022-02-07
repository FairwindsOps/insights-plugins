package commands

import (
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

// RunCommand runs a command and prints errors to Stderr
func ExecInDir(dir string, cmd *exec.Cmd, message string) (string, error) {
	logrus.Info(message)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Errorf("Error running %s - %s[%v]", removeToken(cmd.String()), string(output), removeToken(err.Error()))
	}
	return string(output), err
}

// ExecWithMessage runs a command and prints errors to Stderr
func ExecWithMessage(cmd *exec.Cmd, message string) (string, error) {
	logrus.Info(message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Errorf("Error running %s - %s[%v]", removeToken(cmd.String()), string(output), removeToken(err.Error()))
	}
	return string(output), err
}

// Exec executes a command and returns the results as a string.
func Exec(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	bytes, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Errorf("Unable to execute command: %s %s[%v]", removeToken(cmd.String()), string(bytes), removeToken(err.Error()))
	}
	return string(bytes), err
}

func removeToken(s string) string {
	index := strings.Index(s, "x-access-token") + 15
	index2 := strings.Index(s, "@github.com")
	if index < 0 || index2 < 0 {
		return "<not_able_to_remove_access_token>"
	}
	return strings.ReplaceAll(s, s[index:index2], "<TOKEN>")
}

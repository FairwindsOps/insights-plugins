package commands

import (
	"os/exec"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/util"
	"github.com/sirupsen/logrus"
)

// RunCommand runs a command and prints errors to Stderr
func ExecInDir(dir string, cmd *exec.Cmd, message string) (string, error) {
	logrus.Info(message)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Errorf("Error running %s - %s[%v]", util.RemoveTokensAndPassword(cmd.String()), string(output), util.RemoveTokensAndPassword(err.Error()))
		return "", err
	}
	return string(output), nil
}

// ExecWithMessage runs a command and prints errors to Stderr
func ExecWithMessage(cmd *exec.Cmd, message string) (string, error) {
	logrus.Info(message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Errorf("Error running %s - %s[%v]", util.RemoveTokensAndPassword(cmd.String()), string(output), util.RemoveTokensAndPassword(err.Error()))
		return "", err
	}
	return string(output), nil
}

// Exec executes a command and returns the results as a string.
func Exec(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Errorf("Unable to execute command: %s %s[%v]", util.RemoveTokensAndPassword(cmd.String()), string(output), util.RemoveTokensAndPassword(err.Error()))
		return "", err
	}
	return string(output), nil
}

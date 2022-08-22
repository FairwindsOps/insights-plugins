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
		logrus.Errorf("Error running %s - %s[%v]", util.RemoveToken(cmd.String()), string(output), util.RemoveToken(err.Error()))
	}
	return string(output), err
}

// ExecWithMessage runs a command and prints errors to Stderr
func ExecWithMessage(cmd *exec.Cmd, message string) (string, error) {
	logrus.Info(message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Errorf("Error running %s - %s[%v]", util.RemoveToken(cmd.String()), string(output), util.RemoveToken(err.Error()))
	}
	return string(output), err
}

// Exec executes a command and returns the results as a string.
func Exec(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	bytes, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Errorf("Unable to execute command: %s %s[%v]", util.RemoveToken(cmd.String()), string(bytes), util.RemoveToken(err.Error()))
	}
	return string(bytes), err
}

package commands

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/util"
	"github.com/sirupsen/logrus"
)

// RunCommand runs a command and prints errors to Stderr
func ExecInDir(dir string, cmd *exec.Cmd, message string) (string, error) {
	logrus.Info(message)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		stdoutAndStderr := fmt.Sprintf("%s\n%s", string(stdout.Bytes()), string(stderr.Bytes()))
		logrus.Errorf("Error running %s - %s[%v]", util.RemoveTokensAndPassword(cmd.String()), string(stdoutAndStderr), util.RemoveTokensAndPassword(err.Error()))
		return "", err
	}
	return string(stdout.Bytes()), nil
}

// ExecWithMessage runs a command and prints errors to Stderr
func ExecWithMessage(cmd *exec.Cmd, message string) (string, error) {
	logrus.Info(message)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		stdoutAndStderr := fmt.Sprintf("%s\n%s", string(stdout.Bytes()), string(stderr.Bytes()))
		logrus.Errorf("Error running %s - %s[%v]", util.RemoveTokensAndPassword(cmd.String()), stdoutAndStderr, util.RemoveTokensAndPassword(err.Error()))
		return stdoutAndStderr, err
	}
	return string(stdout.Bytes()), nil
}

// Exec executes a command and returns the results as a string.
func Exec(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		stdoutAndStderr := fmt.Sprintf("%s\n%s", string(stdout.Bytes()), string(stderr.Bytes()))
		logrus.Errorf("Unable to execute command: %s %s[%v]", util.RemoveTokensAndPassword(cmd.String()), stdoutAndStderr, util.RemoveTokensAndPassword(err.Error()))
		return "", err
	}
	return string(stdout.Bytes()), nil
}

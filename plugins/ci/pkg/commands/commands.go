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
		// Vitor - remove access token from cmd
		logrus.Errorf("Error running %s - %s[%v]", cmd, string(output), err)
	}
	return string(output), err
}

// ExecWithMessage runs a command and prints errors to Stderr
func ExecWithMessage(cmd *exec.Cmd, message string) (string, error) {
	logrus.Info(message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Errorf("Error running %s - %s[%v]", cmd, string(output), err)
	}
	return string(output), err
}

// Exec executes a command and returns the results as a string.
func Exec(command string, args ...string) (string, error) {
	bytes, err := exec.Command(command, args...).CombinedOutput()
	if err != nil {
		logrus.Errorf("Unable to execute command: %v %v %s[%v]", command, strings.Join(args, " "), string(bytes), err)
	}
	return string(bytes), err
}

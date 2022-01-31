package commands

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

// RunCommand runs a command and prints errors to Stderr
func ExecInDir(dir string, cmd *exec.Cmd, message string) error {
	logrus.Info(message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputString := string(output)
		logrus.Errorf("Error running %s: %s", cmd, err)
		fmt.Println(outputString)
	}
	return err
}

// ExecWithMessage runs a command and prints errors to Stderr
func ExecWithMessage(cmd *exec.Cmd, message string) error {
	logrus.Info(message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputString := string(output)
		logrus.Errorf("Error running %s: %s", cmd, err)
		fmt.Println(outputString)
	}
	return err
}

// Exec executes a command and returns the results as a string.
func Exec(command string, args ...string) (string, error) {
	bytes, err := exec.Command(command, args...).Output()
	if err != nil {
		logrus.Errorf("Unable to execute command: %v %v", command, strings.Join(args, " "))
		return "", err
	}
	return strings.TrimSpace(string(bytes)), err
}

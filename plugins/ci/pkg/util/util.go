package util

import (
	"fmt"
	"os/exec"

	"github.com/sirupsen/logrus"
)

func RunCommand(cmd *exec.Cmd, message string) error {
	logrus.Info(message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputString := string(output)
		logrus.Errorf("Error running %s: %s", cmd, err)
		fmt.Println(outputString)
	}
	return err
}

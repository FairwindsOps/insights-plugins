package util

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

const UnknownOSMessage = "Unknown OS"

func CheckEnvironmentVariables() error {
	if os.Getenv("FAIRWINDS_INSIGHTS_HOST") == "" || os.Getenv("FAIRWINDS_ORG") == "" || os.Getenv("FAIRWINDS_CLUSTER") == "" || os.Getenv("FAIRWINDS_TOKEN") == "" {
		return errors.New("Proper environment variables not set.")
	}
	return nil
}

func RunCommand(cmd *exec.Cmd, message string) error {
	logrus.Info(message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputString := string(output)
		logrus.Errorf("Error %s: %s\n%s", message, err, outputString)
		if strings.Contains(outputString, UnknownOSMessage) {
			return errors.New(UnknownOSMessage)
		}
	}
	return err
}

package util

import (
	"fmt"
	"os/exec"

	"github.com/sirupsen/logrus"
)

// RunCommand runs a command and prints errors to Stderr
func RunCommandInDir(dir string, cmd *exec.Cmd, message string) error {
	logrus.Info(message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputString := string(output)
		logrus.Errorf("Error running %s: %s", cmd, err)
		fmt.Println(outputString)
	}
	return err
}

// RunCommand runs a command and prints errors to Stderr
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

func ExtractMetadata(obj map[string]interface{}) (string, string, string, string) {
	kind, _ := obj["kind"].(string)
	apiVersion, _ := obj["apiVersion"].(string)
	metadata, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return apiVersion, kind, "", ""
	}
	name, _ := metadata["name"].(string)
	namespace, _ := metadata["namespace"].(string)
	return apiVersion, kind, name, namespace
}

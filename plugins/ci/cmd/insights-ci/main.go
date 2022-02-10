package main

import (
	"os"

	"github.com/fairwindsops/insights-plugins/ci/pkg/ci"
	"github.com/sirupsen/logrus"
)

func main() {
	ciScan, err := ci.NewCIScan()
	if err != nil {
		exitWithError("Error creating CI Scan main struct", err)
	}
	defer ciScan.Close()

	reports, err := ciScan.ProcessRepository()
	if err != nil {
		exitWithError("Error processing repository", err)
	}

	err = ciScan.SendAndPrintResults(reports)
	if err != nil {
		if err == ci.ErrExitCode {
			os.Exit(1)
		}
		exitWithError("Error sending results to insights", err)
	}
}

func exitWithError(message string, err error) {
	if err != nil {
		logrus.Fatalf("%s: %s", message, err.Error())
	} else {
		logrus.Fatal(message)
	}
}

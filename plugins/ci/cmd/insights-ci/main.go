package main

import (
	"os"

	"github.com/fairwindsops/insights-plugins/ci/pkg/ci"
	"github.com/sirupsen/logrus"
)

func main() {
	setLogLevel()
	ciScan, err := ci.NewCIScan()
	if err != nil {
		exitWithError(ciScan, "Error creating CI Scan main struct", err)
	}

	reports, err := ciScan.ProcessRepository()
	if err != nil {
		exitWithError(ciScan, "Error processing repository", err)
	}

	err = ciScan.SendAndPrintResults(reports)
	if err != nil {
		if err == ci.ErrExitCode {
			os.Exit(1)
		}
		exitWithError(ciScan, "Error sending results to insights", err)
	}
	ciScan.Close()
}

func setLogLevel() {
	if os.Getenv("LOGRUS_LEVEL") != "" {
		lvl, err := logrus.ParseLevel(os.Getenv("LOGRUS_LEVEL"))
		if err != nil {
			panic(err)
		}
		logrus.SetLevel(lvl)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}

func exitWithError(ciScan *ci.CIScan, message string, err error) {
	if ciScan != nil {
		ciScan.Close()
	}
	if err != nil {
		logrus.Fatalf("%s: %s", message, err.Error())
	} else {
		logrus.Fatal(message)
	}
}

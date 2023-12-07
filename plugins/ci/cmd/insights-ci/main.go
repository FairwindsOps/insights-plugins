package main

import (
	"os"
	"strings"

	civersion "github.com/fairwindsops/insights-plugins/plugins/ci"
	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/ci"
	"github.com/sirupsen/logrus"
)

func main() {
	setLogLevel()

	// cloneRepo and autoScan are synonymous in this context. they are both used to determine if the repo should be cloned and scanned when running on FW infrastructure.
	cloneRepo := strings.ToLower(strings.TrimSpace(os.Getenv("CLONE_REPO"))) == "true"
	logrus.Infof("cloneRepo: %v", cloneRepo)

	token := strings.TrimSpace(os.Getenv("FAIRWINDS_TOKEN"))
	if token == "" {
		logrus.Fatal("FAIRWINDS_TOKEN environment variable not set")
	}

	logrus.Infof("CI plugin %s", civersion.String())
	ciScan, err := ci.NewCIScan(cloneRepo, token)
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

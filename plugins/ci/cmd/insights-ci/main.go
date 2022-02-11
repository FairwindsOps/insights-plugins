package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	trivymodels "github.com/fairwindsops/insights-plugins/trivy/pkg/models"
	"github.com/sirupsen/logrus"

	"github.com/fairwindsops/insights-plugins/ci/pkg/ci"
	"github.com/fairwindsops/insights-plugins/ci/pkg/models"
)

const maxLinesForPrint = 8

func main() {
	ciScan, err := ci.NewCIScan()
	if err != nil {
		exitWithError(ciScan, "Error creating CI Scan main struct", err)
	}

	err = ciScan.ProcessHelmTemplates()
	if err != nil {
		exitWithError(ciScan, "Error while processing helm templates", err)
	}

	err = ciScan.CopyYaml()
	if err != nil {
		exitWithError(ciScan, "Error while copying YAML files", err)
	}

	// Scan YAML, find all images/kind/etc
	manifestImages, resources, err := ciScan.GetAllResources()
	if err != nil {
		exitWithError(ciScan, "Error while extracting images from YAML manifests", err)
	}

	var reports []models.ReportInfo

	// Scan manifests with Polaris
	if ciScan.PolarisEnabled() {
		polarisReport, err := ciScan.GetPolarisReport()
		if err != nil {
			exitWithError(ciScan, "Error while running Polaris", err)
		}
		reports = append(reports, polarisReport)
	}

	if ciScan.TrivyEnabled() {
		manifestImagesToScan := manifestImages
		if ciScan.SkipTrivyManifests() {
			manifestImagesToScan = []trivymodels.Image{}
		}
		trivyReport, err := ciScan.GetTrivyReport(manifestImagesToScan)
		if err != nil {
			exitWithError(ciScan, "Error while running Trivy", err)
		}
		reports = append(reports, trivyReport)
	}

	workloadReport, err := ciScan.GetWorkloadReport(resources)
	if err != nil {
		exitWithError(ciScan, "Error while aggregating workloads", err)
	}
	reports = append(reports, workloadReport)

	if ciScan.OPAEnabled() {
		opaReport, err := ciScan.ProcessOPA(context.Background())
		if err != nil {
			exitWithError(ciScan, "Error while running OPA", err)
		}
		reports = append(reports, opaReport)
	}

	if ciScan.PlutoEnabled() {
		plutoReport, err := ciScan.GetPlutoReport()
		if err != nil {
			exitWithError(ciScan, "Error while running Pluto", err)
		}
		reports = append(reports, plutoReport)
	}

	results, err := ciScan.SendResults(reports, resources)
	if err != nil {
		exitWithError(ciScan, "Error while sending results back to "+ciScan.Hostname(), err)
	}
	fmt.Printf("%d new Action Items:\n", len(results.NewActionItems))
	printActionItems(results.NewActionItems)
	fmt.Printf("%d fixed Action Items:\n", len(results.FixedActionItems))
	printActionItems(results.FixedActionItems)

	if ciScan.JUnitEnabled() {
		err = ciScan.SaveJUnitFile(*results)
		if err != nil {
			exitWithError(ciScan, "Could not save jUnit results", err)
		}
	}

	if !results.Pass {
		fmt.Printf(
			"\n\nFairwinds Insights checks failed:\n%v\n\nVisit %s/orgs/%s/repositories for more information\n\n",
			err, ciScan.Hostname(), ciScan.Organization())
		if ciScan.ExitCode() {
			os.Exit(1)
		}
	} else {
		fmt.Println("\n\nFairwinds Insights checks passed.")
	}

	ciScan.Close()
}

func printActionItems(ais []models.ActionItem) {
	for _, ai := range ais {
		fmt.Println(ai.GetReadableTitle())
		printMultilineString("Description", ai.Description)
		printMultilineString("Remediation", ai.Remediation)
		fmt.Println()
	}
}

func printMultilineString(title, str string) {
	fmt.Println("  " + title + ":")
	if str == "" {
		str = "Unspecified"
	}
	lines := strings.Split(str, "\n")
	for idx, line := range lines {
		fmt.Println("    " + line)
		if idx == maxLinesForPrint {
			fmt.Println("    [truncated]")
			break
		}
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

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

func exitWithError(message string, err error) {
	if err != nil {
		logrus.Fatalf("%s: %s", message, err.Error())
	} else {
		logrus.Fatal(message)
	}
}

func main() {
	cloneRepo := strings.ToLower(strings.TrimSpace(os.Getenv("CLONE_REPO"))) == "true"
	ci, cleanup, err := ci.NewCI(cloneRepo)
	defer cleanup()
	if err != nil {
		exitWithError(err.Error(), nil)
	}

	err = ci.ProcessHelmTemplates()
	if err != nil {
		exitWithError("Error while processing helm templates", err)
	}

	err = ci.CopyYaml()
	if err != nil {
		exitWithError("Error while copying YAML files", err)
	}

	// Scan YAML, find all images/kind/etc
	manifestImages, resources, err := ci.GetAllResources() // TODO: separate this method into two?
	if err != nil {
		exitWithError("Error while extracting images from YAML manifests", err)
	}

	var reports []models.ReportInfo

	// Scan manifests with Polaris
	if ci.PolarisEnabled() {
		polarisReport, err := ci.GetPolarisReport()
		if err != nil {
			exitWithError("Error while running Polaris", err)
		}
		reports = append(reports, polarisReport)
	}

	if ci.TrivyEnabled() {
		manifestImagesToScan := manifestImages
		if ci.SkipTrivyManifests() {
			manifestImagesToScan = []trivymodels.Image{}
		}
		trivyReport, err := ci.GetTrivyReport(manifestImagesToScan)
		if err != nil {
			exitWithError("Error while running Trivy", err)
		}
		reports = append(reports, trivyReport)
	}

	workloadReport, err := ci.GetWorkloadReport(resources)
	if err != nil {
		exitWithError("Error while aggregating workloads", err)
	}
	reports = append(reports, workloadReport)

	if ci.OPAEnabled() {
		opaReport, err := ci.ProcessOPA(context.Background())
		if err != nil {
			exitWithError("Error while running OPA", err)
		}
		reports = append(reports, opaReport)
	}

	if ci.PlutoEnabled() {
		plutoReport, err := ci.GetPlutoReport()
		if err != nil {
			exitWithError("Error while running Pluto", err)
		}
		reports = append(reports, plutoReport)
	}

	results, err := ci.SendResults(reports, resources)
	if err != nil {
		exitWithError("Error while sending results back to "+ci.Hostname(), err)
	}
	fmt.Printf("%d new Action Items:\n", len(results.NewActionItems))
	printActionItems(results.NewActionItems)
	fmt.Printf("%d fixed Action Items:\n", len(results.FixedActionItems))
	printActionItems(results.FixedActionItems)

	if ci.JUnitEnabled() {
		err = ci.SaveJUnitFile(*results)
		if err != nil {
			exitWithError("Could not save jUnit results", err)
		}
	}

	if !results.Pass {
		fmt.Printf(
			"\n\nFairwinds Insights checks failed:\n%v\n\nVisit %s/orgs/%s/repositories for more information\n\n",
			err, ci.Hostname(), ci.Organization())
		if ci.ExitCode() {
			os.Exit(1)
		}
	} else {
		fmt.Println("\n\nFairwinds Insights checks passed.")
	}
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

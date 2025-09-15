package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/aquasecurity/kube-bench/check"
)

const port = "8080"
const outputFile = "/output/kube-bench.json"
const outputTempFile = "/output/kube-bench-temp.json"

// ID for the Policies category, which is not node specific.
const policiesID = "5"

type kubeBenchModel struct {
	Name     string
	Controls []check.Controls
}

// kubeBenchResponse is the JSON response from the kube-bench command output.
// This is a separate type to decouple the kube-bench CLI API from Insights
// ones, such as the kube-bench aggregator plugin.
type kubeBenchResponse struct {
	Controls []check.Controls
}

var model = kubeBenchModel{}

func getReportsHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(model)
}

func updateModel() {
	logrus.Info("Updating data.")
	cmd := exec.Command("kube-bench", "--json", "--v", "3")

	logrus.Info("Command:", cmd.String())
	logrus.Info("Starting kube-bench execution...")

	// Use CombinedOutput to capture both stdout and stderr
	combinedOutput, err := cmd.CombinedOutput()

	logrus.Info("kube-bench execution completed")
	logrus.Info("Combined output length:", len(combinedOutput))

	if err != nil {
		logrus.Error("Error running kube-bench:", err)
		logrus.Error("Error type:", fmt.Sprintf("%T", err))

		// Try to get more details from the error
		if exitError, ok := err.(*exec.ExitError); ok {
			logrus.Error("Exit code:", exitError.ExitCode())
			logrus.Error("Stderr from exit error:", string(exitError.Stderr))
		}

		// Log the combined output which includes both stdout and stderr
		logrus.Error("Combined output (stdout + stderr):", string(combinedOutput))
		logrus.Fatal("kube-bench failed with detailed error above")
	}

	response := combinedOutput
	logrus.Info("kube-bench output:", string(response))
	decoder := json.NewDecoder(strings.NewReader(string(response)))
	allControls := make([]check.Controls, 0)
	for {
		var controls kubeBenchResponse
		err = decoder.Decode(&controls)
		if err == io.EOF {
			logrus.Info("EOF")
			break
		}
		if err != nil {
			logrus.Error("Error decoding kube-bench output:", err)
			logrus.Fatal(err, string(response))
		}
		allControls = append(allControls, controls.Controls...)
	}
	model.Controls = allControls
	logrus.Info("Data updated.")

}

func main() {
	logrus.Info("Starting:")
	model.Name = os.Getenv("NODE_NAME")
	model.Controls = make([]check.Controls, 0)
	intervalHours := 2
	intervalHoursString := os.Getenv("INTERVAL_HOURS")
	if intervalHoursString != "" {
		var err error
		intervalHours, err = strconv.Atoi(intervalHoursString)
		if err != nil {
			logrus.Fatal(err)
		}
	}
	if strings.ToLower(os.Getenv("RUN_ONCE")) == "true" {
		updateModel()
		data := map[string]check.Controls{}
		for _, control := range model.Controls {
			key := control.ID
			// ID 5 "Policies" should be the same for every node.
			if key != policiesID {
				key = model.Name + "/" + key
			}
			data[key] = control
		}
		outputBytes, err := json.MarshalIndent(data, "", "  ")

		if err != nil {
			panic(err)
		}
		err = os.WriteFile(outputTempFile, outputBytes, 0644)
		if err != nil {
			panic(err)
		}
		err = os.Rename(outputTempFile, outputFile)
		if err != nil {
			panic(err)
		}
		return
	}

	http.HandleFunc("/", getReportsHandler)

	ticker := time.NewTicker(time.Duration(intervalHours) * time.Hour)
	quit := make(chan struct{})
	defer close(quit)
	go func() {
		updateModel()
		for {
			select {
			case <-ticker.C:
				updateModel()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
	logrus.Fatal(http.ListenAndServe(":8080", nil))

}

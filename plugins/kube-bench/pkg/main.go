package main

import (
	"encoding/json"
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
	response, err := cmd.Output()
	if err != nil {
		// If the command failed, try to get stderr for more details
		if exitError, ok := err.(*exec.ExitError); ok {
			logrus.Error("Error running kube-bench:", err)
			logrus.Error("Exit code:", exitError.ExitCode())
			logrus.Error("Stdout:", string(response))
			logrus.Error("Stderr:", string(exitError.Stderr))
			logrus.Fatal("kube-bench failed with detailed error above")
		} else {
			logrus.Error("Error running kube-bench:", err)
			logrus.Fatal(err, string(response))
		}
	}
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

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
const outputFile = "/output/kubesec.json"

type kubeBenchModel struct {
	Name     string
	Controls []check.Controls
}

var model = kubeBenchModel{}

func getReportsHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(model)
}

func updateModel() {
	logrus.Info("Updating data.")
	cmd := exec.Command("kube-bench", "--json")
	response, err := cmd.Output()
	if err != nil {
		logrus.Fatal(err, string(response))
	}
	decoder := json.NewDecoder(strings.NewReader(string(response)))
	allControls := make([]check.Controls, 0)
	for {
		var controls check.Controls
		err = decoder.Decode(&controls)
		if err == io.EOF {
			break
		}
		if err != nil {
			logrus.Fatal(err)
		}
		allControls = append(allControls, controls)
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

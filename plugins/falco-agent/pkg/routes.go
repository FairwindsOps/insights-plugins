package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/fairwindsops/insights-plugins/falco-agent/pkg/data"
	"github.com/sirupsen/logrus"
)

const outputfolder = "/output"
const timestampFormat = "20060102150405"

func inputDataHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("Error reading body: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	outputFile := fmt.Sprintf("%s/%s.json", outputfolder, time.Now().Format(timestampFormat))
	err = ioutil.WriteFile(outputFile, []byte(payload), 0644)
	if err != nil {
		logrus.Errorf("Error writting to file: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write([]byte(`{"status": "ok"}`))
}

func outputDataHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	payload, err := data.Aggregate24hrsData(outputfolder)
	if err != nil {
		logrus.Errorf("Error while aggregating data: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	output := data.OutputFormat{
		Output: payload,
	}
	data, err := json.Marshal(output)
	if err != nil {
		logrus.Errorf("Error on json.Marshal(payload): %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = w.Write([]byte(data))
	if err != nil {
		logrus.Errorf("Error while sending data: %v", err)
	}
}

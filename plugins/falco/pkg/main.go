package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const port = "3031"
const outputfolder = "/output"
const timestampFormat = "20060102150405"

func falcoDataHandler(w http.ResponseWriter, r *http.Request) {
	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logrus.Infof("Error reading body: %v", err)
		http.Error(w, "can't read body", http.StatusBadRequest)
	}

	outputFile := fmt.Sprintf("%s/%s.json", outputfolder, time.Now().Format(timestampFormat))
	err = ioutil.WriteFile(outputFile, []byte(payload), 0644)
	if err != nil {
		panic(err)
	}
	w.Write([]byte(`{"status": "ok"}`))
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/data", falcoDataHandler).Methods(http.MethodPost)
	srv := &http.Server{
		Handler: r,
		Addr:    fmt.Sprintf(":%s", port),
	}
	logrus.Infof("server is running at http://localhost:%s", port)
	logrus.Fatal(srv.ListenAndServe())
}

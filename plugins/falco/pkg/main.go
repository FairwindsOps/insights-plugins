package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const port = "3031"

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/data", inputDataHandler).Methods(http.MethodPost)
	r.HandleFunc("/output", outputDataHandler).Methods(http.MethodGet)
	srv := &http.Server{
		Handler: r,
		Addr:    fmt.Sprintf(":%s", port),
	}
	logrus.Infof("server is running at http://0.0.0.0:%s", port)
	logrus.Fatal(srv.ListenAndServe())
}

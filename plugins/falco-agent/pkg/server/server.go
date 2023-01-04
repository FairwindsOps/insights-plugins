package server

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

)

func CreateServer(port int) (*http.Server, error) {
	r := mux.NewRouter()

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	r.HandleFunc("/data", inputDataHandler).Methods(http.MethodPost)

	r.HandleFunc("/output", outputDataHandler).Methods(http.MethodGet)

	srv := &http.Server{
		Handler: r,
		Addr:    fmt.Sprintf(":%d", port),
	}

	return srv, nil
}

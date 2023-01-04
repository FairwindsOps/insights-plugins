package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/fairwindsops/insights-plugins/plugins/falco-agent/pkg/kube"
)

func CreateServer(port int) (*http.Server, error) {
	dynamic, restMapper, err := kube.GetKubeClient()
	if err != nil {
		return nil, err
	}

	r := mux.NewRouter()

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	r.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		inputDataHandler(w, r, context.Background(), dynamic, restMapper)
	}).Methods(http.MethodPost)

	r.HandleFunc("/output", outputDataHandler).Methods(http.MethodGet)

	srv := &http.Server{
		Handler: r,
		Addr:    fmt.Sprintf(":%d", port),
	}

	return srv, nil
}

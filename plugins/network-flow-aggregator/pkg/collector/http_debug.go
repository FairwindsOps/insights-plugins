package collector

import (
	"encoding/json"
	"net/http"

	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/store"
)

type DebugHTTPServer struct {
	store *store.Store
}

func NewDebugHTTPServer(st *store.Store) *DebugHTTPServer {
	return &DebugHTTPServer{store: st}
}

func (h *DebugHTTPServer) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /api/v1/servicemap", h.handleServiceMap)
}

func (h *DebugHTTPServer) handleServiceMap(w http.ResponseWriter, _ *http.Request) {
	edges := h.store.ListEdges()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"edges": edges,
		"count": len(edges),
	})
}

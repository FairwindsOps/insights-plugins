package collector

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/store"
	insightsv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/insights/v1"
	"google.golang.org/protobuf/encoding/protojson"
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
	mux.HandleFunc("GET /api/v1/flows", h.handleFlows)
}

func (h *DebugHTTPServer) handleFlows(w http.ResponseWriter, r *http.Request) {
	opts := store.ListOpts{
		Since:           parseInt64(r.URL.Query().Get("since")),
		Limit:           parseInt(r.URL.Query().Get("limit")),
		Offset:          parseInt(r.URL.Query().Get("offset")),
		Namespace:       r.URL.Query().Get("namespace"),
		SrcWorkloadKind: r.URL.Query().Get("src_workload_kind"),
		DstKind:         r.URL.Query().Get("dst_kind"),
	}
	if kind := r.URL.Query().Get("event_kind"); kind != "" {
		opts.EventKind = parseEventKind(kind)
	}

	events := h.store.ListEvents(opts)
	marshaler := protojson.MarshalOptions{EmitUnpopulated: false}
	out := make([]json.RawMessage, 0, len(events))
	for _, event := range events {
		if event == nil {
			continue
		}
		b, err := marshaler.Marshal(event)
		if err != nil {
			http.Error(w, "marshal event", http.StatusInternalServerError)
			return
		}
		out = append(out, json.RawMessage(b))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"events": out,
		"count":  len(out),
	})
}

func parseInt64(v string) int64 {
	if v == "" {
		return 0
	}
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

func parseInt(v string) int {
	if v == "" {
		return 0
	}
	n, _ := strconv.Atoi(v)
	return n
}

func parseEventKind(v string) insightsv1.FlowEventKind {
	switch v {
	case "CONNECT", "connect", "1":
		return insightsv1.FlowEventKind_FLOW_EVENT_KIND_CONNECT
	case "TRAFFIC", "traffic", "2":
		return insightsv1.FlowEventKind_FLOW_EVENT_KIND_TRAFFIC
	case "DNS_QUERY", "dns_query", "3":
		return insightsv1.FlowEventKind_FLOW_EVENT_KIND_DNS_QUERY
	case "DNS_RESPONSE", "dns_response", "4":
		return insightsv1.FlowEventKind_FLOW_EVENT_KIND_DNS_RESPONSE
	default:
		return insightsv1.FlowEventKind_FLOW_EVENT_KIND_UNSPECIFIED
	}
}

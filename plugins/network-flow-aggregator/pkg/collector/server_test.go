package collector

import (
	"log/slog"
	"testing"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/dns"
	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/store"
	aggregv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/aggregator/v1"
)

func TestEnrichEventTCPUsesDNSCache(t *testing.T) {
	cache := dns.NewCache(time.Minute)
	s := &Server{dnsCache: cache, log: slog.Default()}

	cache.RecordResponse("default", "frontend", "example.com", "A", "Success", []string{"93.184.216.34"}, time.Now())

	enrich := s.enrichEvent(&aggregv1.FlowEvent{
		EventKind:         aggregv1.FlowEventKind_FLOW_EVENT_KIND_CONNECT,
		Protocol:          aggregv1.Protocol_PROTOCOL_TCP,
		TimestampUnixNano: time.Now().UnixNano(),
		Src:               &aggregv1.WorkloadRef{Namespace: "default", Pod: "frontend"},
		Dst:               &aggregv1.Endpoint{Addr: "93.184.216.34", Port: 443},
	})
	if enrich.DstKind != "ExternalHostname" || enrich.DstName != "example.com" {
		t.Fatalf("enrichment = %+v", enrich)
	}
}

func TestEnrichEventDNSResponseRecordsCache(t *testing.T) {
	cache := dns.NewCache(time.Minute)
	s := &Server{dnsCache: cache, log: slog.Default()}

	s.enrichEvent(&aggregv1.FlowEvent{
		EventKind:         aggregv1.FlowEventKind_FLOW_EVENT_KIND_DNS_RESPONSE,
		Protocol:          aggregv1.Protocol_PROTOCOL_DNS,
		TimestampUnixNano: time.Now().UnixNano(),
		Src:               &aggregv1.WorkloadRef{Namespace: "default", Pod: "frontend"},
		Dst:               &aggregv1.Endpoint{Addr: "93.184.216.34", Port: 443},
		Dns: &aggregv1.DnsDetails{
			Name:      "example.com",
			Qtype:     "A",
			Rcode:     "Success",
			Addresses: []string{"93.184.216.34"},
		},
	})

	host, ok := cache.Lookup("default", "frontend", "93.184.216.34")
	if !ok || host != "example.com" {
		t.Fatalf("cache lookup = %q ok=%v", host, ok)
	}
}

func TestStoreAcceptsDNSEventWithoutDstAddr(t *testing.T) {
	st := store.NewStore(100, time.Minute)
	accepted, _ := st.AppendBatch(&aggregv1.FlowEventBatch{
		NodeName: "node-a",
		AgentId:  "agent-a",
		Events: []*aggregv1.FlowEvent{
			{
				EventKind:         aggregv1.FlowEventKind_FLOW_EVENT_KIND_DNS_QUERY,
				Protocol:          aggregv1.Protocol_PROTOCOL_DNS,
				TimestampUnixNano: time.Now().UnixNano(),
				Src:               &aggregv1.WorkloadRef{Namespace: "default", Pod: "frontend"},
				Dst:               &aggregv1.Endpoint{Addr: "10.96.0.10", Port: 53},
				Dns:               &aggregv1.DnsDetails{Name: "example.com", Qtype: "A"},
			},
		},
	}, nil)
	if accepted != 1 {
		t.Fatalf("accepted = %d", accepted)
	}
}

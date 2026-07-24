package collector

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/dns"
	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/kube"
	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/peerindex"
	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/store"
	aggregv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/aggregator/v1"
)

type mockFlowEnricher struct {
	endpoints map[string]kube.EndpointEntry
	dstByKey  map[string]kube.DstIdentity
	srcByKey  map[string]kube.WorkloadIdentity
}

func (m *mockFlowEnricher) ResolveSrcWorkload(namespace, podName string) kube.WorkloadIdentity {
	if m.srcByKey != nil {
		if wl, ok := m.srcByKey[fmt.Sprintf("%s/%s", namespace, podName)]; ok {
			return wl
		}
	}
	if namespace == "payments" && podName == "backend-6f9c48f647-vnbzk" {
		return kube.WorkloadIdentity{Namespace: "payments", Kind: "Deployment", Name: "backend"}
	}
	return kube.WorkloadIdentity{Namespace: namespace, Kind: "Pod", Name: podName}
}

func (m *mockFlowEnricher) ResolveDst(addr string, port uint32) kube.DstIdentity {
	if m.dstByKey != nil {
		if dst, ok := m.dstByKey[fmt.Sprintf("%s:%d", addr, port)]; ok {
			return dst
		}
	}
	if port == 8080 && (addr == "10.96.89.41" || addr == "10.244.0.95") {
		return kube.DstIdentity{Namespace: "payments", Kind: "Service", Name: "backend", Addr: addr}
	}
	return kube.DstIdentity{Addr: addr}
}

func (m *mockFlowEnricher) LookupEndpoint(addr string, port uint32) (kube.EndpointEntry, bool) {
	if m.endpoints == nil {
		return kube.EndpointEntry{}, false
	}
	entry, ok := m.endpoints[fmt.Sprintf("%s:%d", addr, port)]
	return entry, ok
}

func TestEnrichEventClientServiceUsesPeerIndex(t *testing.T) {
	mock := &mockFlowEnricher{
		endpoints: map[string]kube.EndpointEntry{
			"10.244.0.95:8080": {
				ServiceNamespace: "payments",
				ServiceName:      "backend",
				PodNamespace:     "payments",
				PodName:          "backend-6f9c48f647-vnbzk",
			},
		},
	}
	s := &Server{
		enricher:  mock,
		peerIndex: peerindex.New(time.Minute),
		log:       slog.Default(),
	}

	now := time.Now()
	s.recordServerPeer(&aggregv1.FlowEvent{
		EventKind:         aggregv1.FlowEventKind_FLOW_EVENT_KIND_TRAFFIC,
		Protocol:          aggregv1.Protocol_PROTOCOL_TCP,
		TimestampUnixNano: now.UnixNano(),
		Src:               &aggregv1.WorkloadRef{Namespace: "payments", Pod: "backend-6f9c48f647-vnbzk"},
		SrcEndpoint:       &aggregv1.Endpoint{Addr: "10.244.0.95", Port: 8080},
		Dst:               &aggregv1.Endpoint{Addr: "10.244.0.94", Port: 55444},
	}, mock.ResolveSrcWorkload("payments", "backend-6f9c48f647-vnbzk"))

	serverEnrich := s.enrichEvent(&aggregv1.FlowEvent{
		EventKind:         aggregv1.FlowEventKind_FLOW_EVENT_KIND_TRAFFIC,
		Protocol:          aggregv1.Protocol_PROTOCOL_TCP,
		TimestampUnixNano: now.UnixNano(),
		Src:               &aggregv1.WorkloadRef{Namespace: "payments", Pod: "backend-6f9c48f647-vnbzk"},
		SrcEndpoint:       &aggregv1.Endpoint{Addr: "10.244.0.95", Port: 8080},
		Dst:               &aggregv1.Endpoint{Addr: "10.244.0.94", Port: 55444},
		BytesSent:         191,
		BytesReceived:     143,
	})
	if serverEnrich.BackendPodName != "" {
		t.Fatalf("server-side event should not get backend attribution: %+v", serverEnrich)
	}

	clientEnrich := s.enrichEvent(&aggregv1.FlowEvent{
		EventKind:         aggregv1.FlowEventKind_FLOW_EVENT_KIND_TRAFFIC,
		Protocol:          aggregv1.Protocol_PROTOCOL_TCP,
		TimestampUnixNano: now.UnixNano(),
		Src:               &aggregv1.WorkloadRef{Namespace: "payments", Pod: "frontend-abc"},
		SrcEndpoint:       &aggregv1.Endpoint{Addr: "10.244.0.94", Port: 55444},
		Dst:               &aggregv1.Endpoint{Addr: "10.96.89.41", Port: 8080},
		BytesSent:         143,
		BytesReceived:     191,
	})
	if clientEnrich.BackendPodName != "backend-6f9c48f647-vnbzk" {
		t.Fatalf("client enrichment = %+v", clientEnrich)
	}
	if clientEnrich.BackendWorkloadKind != "Deployment" || clientEnrich.BackendWorkloadName != "backend" {
		t.Fatalf("backend workload = %+v", clientEnrich)
	}
}

func TestEnrichEventClientServiceDirectPodIP(t *testing.T) {
	mock := &mockFlowEnricher{
		endpoints: map[string]kube.EndpointEntry{
			"10.244.0.95:8080": {
				ServiceNamespace: "payments",
				ServiceName:      "backend",
				PodNamespace:     "payments",
				PodName:          "backend-6f9c48f647-vnbzk",
			},
		},
	}
	s := &Server{enricher: mock, peerIndex: peerindex.New(time.Minute), log: slog.Default()}

	enrich := s.enrichEvent(&aggregv1.FlowEvent{
		EventKind:         aggregv1.FlowEventKind_FLOW_EVENT_KIND_CONNECT,
		Protocol:          aggregv1.Protocol_PROTOCOL_TCP,
		TimestampUnixNano: time.Now().UnixNano(),
		Src:               &aggregv1.WorkloadRef{Namespace: "payments", Pod: "frontend-abc"},
		SrcEndpoint:       &aggregv1.Endpoint{Addr: "10.244.0.94", Port: 55444},
		Dst:               &aggregv1.Endpoint{Addr: "10.244.0.95", Port: 8080},
	})
	if enrich.BackendPodName != "backend-6f9c48f647-vnbzk" {
		t.Fatalf("enrichment = %+v", enrich)
	}
}

func TestEnrichEventRejectsMismatchedPeerService(t *testing.T) {
	mock := &mockFlowEnricher{endpoints: map[string]kube.EndpointEntry{}}
	s := &Server{enricher: mock, peerIndex: peerindex.New(time.Minute), log: slog.Default()}
	s.peerIndex.Put("10.244.0.94", 55444, kube.BackendIdentity{
		PodNamespace:      "accounting",
		PodName:           "backend-other",
		WorkloadNamespace: "accounting",
		WorkloadKind:      "Deployment",
		WorkloadName:    "backend",
		ServiceNamespace:  "accounting",
		ServiceName:       "backend",
	}, time.Now())

	enrich := s.enrichEvent(&aggregv1.FlowEvent{
		EventKind:         aggregv1.FlowEventKind_FLOW_EVENT_KIND_CONNECT,
		Protocol:          aggregv1.Protocol_PROTOCOL_TCP,
		TimestampUnixNano: time.Now().UnixNano(),
		Src:               &aggregv1.WorkloadRef{Namespace: "payments", Pod: "frontend-abc"},
		SrcEndpoint:       &aggregv1.Endpoint{Addr: "10.244.0.94", Port: 55444},
		Dst:               &aggregv1.Endpoint{Addr: "10.96.89.41", Port: 8080},
	})
	if enrich.BackendPodName != "" {
		t.Fatalf("expected no backend for mismatched service, got %+v", enrich)
	}
}

func TestStorePersistsBackendFields(t *testing.T) {
	st := store.NewStore(100, time.Minute)
	batch := &aggregv1.FlowEventBatch{
		Events: []*aggregv1.FlowEvent{
			{
				EventKind:         aggregv1.FlowEventKind_FLOW_EVENT_KIND_TRAFFIC,
				Protocol:          aggregv1.Protocol_PROTOCOL_TCP,
				TimestampUnixNano: time.Now().UnixNano(),
				Src:               &aggregv1.WorkloadRef{Namespace: "payments", Pod: "frontend-abc"},
				SrcEndpoint:       &aggregv1.Endpoint{Addr: "10.244.0.94", Port: 55444},
				Dst:               &aggregv1.Endpoint{Addr: "10.96.89.41", Port: 8080},
				BytesSent:         143,
				BytesReceived:     191,
			},
		},
	}
	enrich := func(_ *aggregv1.FlowEvent) store.Enrichment {
		return store.Enrichment{
			SrcNamespace:             "payments",
			SrcWorkloadKind:          "Deployment",
			SrcWorkloadName:          "frontend",
			DstNamespace:             "payments",
			DstKind:                  "Service",
			DstName:                  "backend",
			BackendWorkloadNamespace: "payments",
			BackendWorkloadKind:      "Deployment",
			BackendWorkloadName:      "backend",
			BackendPodNamespace:      "payments",
			BackendPodName:           "backend-6f9c48f647-vnbzk",
		}
	}
	if got, _ := st.AppendBatch(batch, enrich); got != 1 {
		t.Fatalf("accepted = %d", got)
	}
	events := st.ListEvents(store.ListOpts{})
	if events[0].GetBackendPod().GetPod() != "backend-6f9c48f647-vnbzk" {
		t.Fatalf("backend pod = %#v", events[0].GetBackendPod())
	}
	if events[0].GetBackendWorkload().GetName() != "backend" {
		t.Fatalf("backend workload = %#v", events[0].GetBackendWorkload())
	}
}

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

func TestEnrichEventBlanksDstOnServerObservedReverse(t *testing.T) {
	// Server reverse with LookupEndpoint(src) + recycled client IP wrongly labeled as CronJob.
	mock := &mockFlowEnricher{
		endpoints: map[string]kube.EndpointEntry{
			"10.244.0.50:5432": {
				ServiceNamespace: "insights",
				ServiceName:      "insights-timescale-rw",
				PodNamespace:     "insights",
				PodName:          "timescale-0",
			},
		},
		dstByKey: map[string]kube.DstIdentity{
			"10.96.0.20:5432": {
				Namespace: "insights",
				Kind:      "Service",
				Name:      "insights-timescale-rw",
				Addr:      "10.96.0.20",
			},
			"10.244.0.94:39620": {
				Namespace: "fwinsights",
				Kind:      "CronJob",
				Name:      "action-items-statistics",
				Addr:      "10.244.0.94",
			},
		},
		srcByKey: map[string]kube.WorkloadIdentity{
			"insights/timescale-0": {Namespace: "insights", Kind: "StatefulSet", Name: "timescale"},
			"fwinsights/img-vulns-pod": {
				Namespace: "fwinsights",
				Kind:      "CronJob",
				Name:      "img-vulns-on-demand-refresh",
			},
		},
	}
	s := &Server{enricher: mock, peerIndex: peerindex.New(time.Minute), log: slog.Default()}
	now := time.Now()

	reverse := s.enrichEvent(&aggregv1.FlowEvent{
		EventKind:         aggregv1.FlowEventKind_FLOW_EVENT_KIND_TRAFFIC,
		Protocol:          aggregv1.Protocol_PROTOCOL_TCP,
		TimestampUnixNano: now.UnixNano(),
		Src:               &aggregv1.WorkloadRef{Namespace: "insights", Pod: "timescale-0"},
		SrcEndpoint:       &aggregv1.Endpoint{Addr: "10.244.0.50", Port: 5432},
		Dst:               &aggregv1.Endpoint{Addr: "10.244.0.94", Port: 39620},
	})
	if reverse.DstKind != "" || reverse.DstName != "" {
		t.Fatalf("reverse dst should be blank, got %+v", reverse)
	}
	if reverse.SrcWorkloadKind != "StatefulSet" || reverse.SrcWorkloadName != "timescale" {
		t.Fatalf("reverse src = %+v", reverse)
	}

	client := s.enrichEvent(&aggregv1.FlowEvent{
		EventKind:         aggregv1.FlowEventKind_FLOW_EVENT_KIND_TRAFFIC,
		Protocol:          aggregv1.Protocol_PROTOCOL_TCP,
		TimestampUnixNano: now.UnixNano(),
		Src:               &aggregv1.WorkloadRef{Namespace: "fwinsights", Pod: "img-vulns-pod"},
		SrcEndpoint:       &aggregv1.Endpoint{Addr: "10.244.0.94", Port: 39620},
		Dst:               &aggregv1.Endpoint{Addr: "10.96.0.20", Port: 5432},
	})
	if client.DstKind != "Service" || client.DstName != "insights-timescale-rw" {
		t.Fatalf("client dst = %+v", client)
	}
	if client.BackendPodName != "timescale-0" {
		t.Fatalf("peer-index backend after reverse = %+v", client)
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

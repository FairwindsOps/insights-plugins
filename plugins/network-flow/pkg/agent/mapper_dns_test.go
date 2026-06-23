package agent

import (
	"testing"

	flowv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow/pkg/flow/v1"
)

func TestMapDnsEventQuery(t *testing.T) {
	event := MapDnsEvent(DNSFields{
		Namespace: "default",
		Pod:       "frontend",
		Container: "app",
		SrcAddr:   "10.0.0.2",
		SrcPort:   45678,
		DstAddr:   "10.96.0.10",
		DstPort:   53,
		Timestamp: 123,
		QR:        "Q",
		Name:      "api.stripe.com.",
		QType:     "A",
		QueryID:   42,
	})
	if event == nil {
		t.Fatal("expected event")
	}
	if event.GetEventKind() != flowv1.FlowEventKind_FLOW_EVENT_KIND_DNS_QUERY {
		t.Fatalf("event_kind = %v", event.GetEventKind())
	}
	if event.GetProtocol() != flowv1.Protocol_PROTOCOL_DNS {
		t.Fatalf("protocol = %v", event.GetProtocol())
	}
	if event.GetDns().GetName() != "api.stripe.com" {
		t.Fatalf("name = %q", event.GetDns().GetName())
	}
	if event.GetDns().GetQueryId() != 42 {
		t.Fatalf("query_id = %d", event.GetDns().GetQueryId())
	}
	if event.GetDst().GetAddr() != "10.96.0.10" {
		t.Fatalf("dst addr = %q", event.GetDst().GetAddr())
	}
}

func TestMapDnsEventResponse(t *testing.T) {
	event := MapDnsEvent(DNSFields{
		Namespace: "default",
		Pod:       "frontend",
		SrcAddr:   "10.96.0.10",
		SrcPort:   53,
		DstAddr:   "10.0.0.2",
		DstPort:   45678,
		Timestamp: 456,
		QR:        "R",
		Name:      "api.stripe.com",
		QType:     "A",
		RCode:     "Success",
		Addresses: "104.21.11.16, 104.21.12.16",
		QueryID:   42,
	})
	if event == nil {
		t.Fatal("expected event")
	}
	if event.GetEventKind() != flowv1.FlowEventKind_FLOW_EVENT_KIND_DNS_RESPONSE {
		t.Fatalf("event_kind = %v", event.GetEventKind())
	}
	if event.GetDst().GetAddr() != "104.21.11.16" {
		t.Fatalf("dst addr = %q", event.GetDst().GetAddr())
	}
	addrs := event.GetDns().GetAddresses()
	if len(addrs) != 2 || addrs[0] != "104.21.11.16" || addrs[1] != "104.21.12.16" {
		t.Fatalf("addresses = %v", addrs)
	}
}

func TestMapDnsEventSkipsIncomplete(t *testing.T) {
	if MapDnsEvent(DNSFields{Pod: "x", Name: "example.com", QR: "Q"}) == nil {
		t.Fatal("expected event with pod and name")
	}
	if MapDnsEvent(DNSFields{Pod: "x", QR: "Q"}) != nil {
		t.Fatal("expected nil without name")
	}
	if MapDnsEvent(DNSFields{Name: "example.com", QR: "Q"}) != nil {
		t.Fatal("expected nil without pod")
	}
}

func TestNormalizeHostname(t *testing.T) {
	if got := normalizeHostname("example.com."); got != "example.com" {
		t.Fatalf("got %q", got)
	}
}

func TestParseAddresses(t *testing.T) {
	addrs := parseAddresses("1.2.3.4, 5.6.7.8")
	if len(addrs) != 2 {
		t.Fatalf("got %v", addrs)
	}
	if parseAddresses("") != nil {
		t.Fatal("expected nil for empty")
	}
}

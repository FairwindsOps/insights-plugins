package agent

import (
	"testing"

	flowv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow/pkg/flow/v1"
)

func TestMapFlowEventConnect(t *testing.T) {
	event := MapFlowEvent(TCPFields{
		Namespace: "default",
		Pod:       "frontend",
		Container: "app",
		SrcAddr:   "10.0.0.2",
		SrcPort:   45678,
		DstAddr:   "10.0.0.5",
		DstPort:   443,
		Timestamp: 123,
		EventKind: flowv1.FlowEventKind_FLOW_EVENT_KIND_CONNECT,
	})
	if event == nil {
		t.Fatal("expected event")
	}
	if event.GetEventKind() != flowv1.FlowEventKind_FLOW_EVENT_KIND_CONNECT {
		t.Fatalf("event_kind = %v", event.GetEventKind())
	}
	if event.GetProtocol() != flowv1.Protocol_PROTOCOL_TCP {
		t.Fatalf("protocol = %v", event.GetProtocol())
	}
	if event.GetSrc().GetPod() != "frontend" {
		t.Fatalf("pod = %q", event.GetSrc().GetPod())
	}
	if event.GetSrcEndpoint().GetPort() != 45678 {
		t.Fatalf("src port = %d", event.GetSrcEndpoint().GetPort())
	}
	if event.GetDst().GetPort() != 443 {
		t.Fatalf("dst port = %d", event.GetDst().GetPort())
	}
}

func TestMapFlowEventTraffic(t *testing.T) {
	event := MapFlowEvent(TCPFields{
		Namespace:     "default",
		Pod:           "frontend",
		SrcAddr:       "10.0.0.2",
		SrcPort:       45678,
		DstAddr:       "10.0.0.5",
		DstPort:       443,
		Timestamp:     123,
		BytesSent:     10,
		BytesReceived: 20,
		EventKind:     flowv1.FlowEventKind_FLOW_EVENT_KIND_TRAFFIC,
	})
	if event == nil {
		t.Fatal("expected event")
	}
	if event.GetEventKind() != flowv1.FlowEventKind_FLOW_EVENT_KIND_TRAFFIC {
		t.Fatalf("event_kind = %v", event.GetEventKind())
	}
	if event.GetBytesSent() != 10 || event.GetBytesReceived() != 20 {
		t.Fatalf("bytes = sent:%d recv:%d", event.GetBytesSent(), event.GetBytesReceived())
	}
}

func TestMapFlowEventSkipsIncomplete(t *testing.T) {
	if MapFlowEvent(TCPFields{Pod: "x", EventKind: flowv1.FlowEventKind_FLOW_EVENT_KIND_CONNECT}) != nil {
		t.Fatal("expected nil without dst")
	}
	if MapFlowEvent(TCPFields{DstAddr: "1.1.1.1", EventKind: flowv1.FlowEventKind_FLOW_EVENT_KIND_CONNECT}) != nil {
		t.Fatal("expected nil without pod")
	}
}

func TestSplitEndpoint(t *testing.T) {
	addr, port := splitEndpoint("10.0.0.1:8080")
	if addr != "10.0.0.1" || port != 8080 {
		t.Fatalf("got %q:%d", addr, port)
	}
}

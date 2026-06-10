package agent

import (
	"testing"

	flowv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow/pkg/flow/v1"
)

func TestMapTCP(t *testing.T) {
	flow := MapTCP(TCPFields{
		Namespace:     "default",
		Pod:           "frontend",
		Container:     "app",
		DstAddr:       "10.0.0.5",
		DstPort:       443,
		Timestamp:     123,
		BytesSent:     10,
		BytesReceived: 20,
	})
	if flow == nil {
		t.Fatal("expected flow")
	}
	if flow.GetType() != flowv1.FlowType_FLOW_TYPE_TCP {
		t.Fatalf("type = %v", flow.GetType())
	}
	if flow.GetSrc().GetPod() != "frontend" {
		t.Fatalf("pod = %q", flow.GetSrc().GetPod())
	}
	if flow.GetDst().GetPort() != 443 {
		t.Fatalf("port = %d", flow.GetDst().GetPort())
	}
	if flow.GetBytesSent() != 10 || flow.GetBytesReceived() != 20 {
		t.Fatalf("bytes = sent:%d recv:%d", flow.GetBytesSent(), flow.GetBytesReceived())
	}
}

func TestMapTCPSkipsIncomplete(t *testing.T) {
	if MapTCP(TCPFields{Pod: "x"}) != nil {
		t.Fatal("expected nil without dst")
	}
	if MapTCP(TCPFields{DstAddr: "1.1.1.1"}) != nil {
		t.Fatal("expected nil without pod")
	}
}

func TestSplitEndpoint(t *testing.T) {
	addr, port := splitEndpoint("10.0.0.1:8080")
	if addr != "10.0.0.1" || port != 8080 {
		t.Fatalf("got %q:%d", addr, port)
	}
}

package agent

import (
	"fmt"
	"strings"

	flowv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow/pkg/flow/v1"
)

type TCPFields struct {
	Namespace     string
	Pod           string
	Container     string
	SrcAddr       string
	SrcPort       uint32
	DstAddr       string
	DstPort       uint32
	PID           uint32
	Timestamp     int64
	BytesSent     uint64
	BytesReceived uint64
}

func MapTCP(fields TCPFields) *flowv1.NetworkFlow {
	if fields.Pod == "" || fields.DstAddr == "" {
		return nil
	}
	return &flowv1.NetworkFlow{
		Type:              flowv1.FlowType_FLOW_TYPE_TCP,
		TimestampUnixNano: fields.Timestamp,
		Src: &flowv1.WorkloadEndpoint{
			Namespace: fields.Namespace,
			Pod:       fields.Pod,
			Container: fields.Container,
		},
		Dst: &flowv1.NetworkEndpoint{
			Addr: fields.DstAddr,
			Port: fields.DstPort,
		},
		BytesSent:     fields.BytesSent,
		BytesReceived: fields.BytesReceived,
	}
}

func splitEndpoint(endpoint string) (string, uint32) {
	if endpoint == "" {
		return "", 0
	}
	if i := strings.LastIndex(endpoint, ":"); i > 0 {
		addr := endpoint[:i]
		var port uint32
		fmt.Sscanf(endpoint[i+1:], "%d", &port)
		return addr, port
	}
	return endpoint, 0
}

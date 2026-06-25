package agent

import (
	"fmt"
	"strings"

	aggregv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/aggregator/v1"
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
	EventKind     aggregv1.FlowEventKind
}

func MapFlowEvent(fields TCPFields) *aggregv1.FlowEvent {
	if fields.Pod == "" || fields.DstAddr == "" {
		return nil
	}
	return &aggregv1.FlowEvent{
		EventKind:         fields.EventKind,
		Protocol:          aggregv1.Protocol_PROTOCOL_TCP,
		TimestampUnixNano: fields.Timestamp,
		Src: &aggregv1.WorkloadRef{
			Namespace: fields.Namespace,
			Pod:       fields.Pod,
			Container: fields.Container,
		},
		SrcEndpoint: &aggregv1.Endpoint{
			Addr: fields.SrcAddr,
			Port: fields.SrcPort,
		},
		Dst: &aggregv1.Endpoint{
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

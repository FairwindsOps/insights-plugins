package agent

import (
	"strings"

	flowv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow/pkg/flow/v1"
)

type DNSFields struct {
	Namespace string
	Pod       string
	Container string
	SrcAddr   string
	SrcPort   uint32
	DstAddr   string
	DstPort   uint32
	Timestamp int64
	QR        string
	Name      string
	QType     string
	RCode     string
	Addresses string
	QueryID   uint32
}

func MapDnsEvent(fields DNSFields) *flowv1.FlowEvent {
	if fields.Pod == "" || fields.Name == "" {
		return nil
	}

	kind := flowv1.FlowEventKind_FLOW_EVENT_KIND_DNS_QUERY
	if fields.QR == "R" {
		kind = flowv1.FlowEventKind_FLOW_EVENT_KIND_DNS_RESPONSE
	}

	addresses := parseAddresses(fields.Addresses)
	dstAddr := fields.DstAddr
	dstPort := fields.DstPort
	if kind == flowv1.FlowEventKind_FLOW_EVENT_KIND_DNS_RESPONSE && len(addresses) > 0 {
		dstAddr = addresses[0]
	}

	return &flowv1.FlowEvent{
		EventKind:         kind,
		Protocol:          flowv1.Protocol_PROTOCOL_DNS,
		TimestampUnixNano: fields.Timestamp,
		Src: &flowv1.WorkloadRef{
			Namespace: fields.Namespace,
			Pod:       fields.Pod,
			Container: fields.Container,
		},
		SrcEndpoint: &flowv1.Endpoint{
			Addr: fields.SrcAddr,
			Port: fields.SrcPort,
		},
		Dst: &flowv1.Endpoint{
			Addr: dstAddr,
			Port: dstPort,
		},
		Dns: &flowv1.DnsDetails{
			Name:      normalizeHostname(fields.Name),
			Qtype:     fields.QType,
			Rcode:     fields.RCode,
			Addresses: addresses,
			QueryId:   fields.QueryID,
		},
	}
}

func normalizeHostname(name string) string {
	return strings.TrimSuffix(strings.TrimSpace(name), ".")
}

func parseAddresses(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

package agent

import (
	"strings"

	aggregv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/aggregator/v1"
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

func MapDnsEvent(fields DNSFields) *aggregv1.FlowEvent {
	if fields.Pod == "" || fields.Name == "" {
		return nil
	}

	kind := aggregv1.FlowEventKind_FLOW_EVENT_KIND_DNS_QUERY
	if fields.QR == "R" {
		kind = aggregv1.FlowEventKind_FLOW_EVENT_KIND_DNS_RESPONSE
	}

	addresses := parseAddresses(fields.Addresses)
	dstAddr := fields.DstAddr
	dstPort := fields.DstPort
	if kind == aggregv1.FlowEventKind_FLOW_EVENT_KIND_DNS_RESPONSE && len(addresses) > 0 {
		dstAddr = addresses[0]
	}

	return &aggregv1.FlowEvent{
		EventKind:         kind,
		Protocol:          aggregv1.Protocol_PROTOCOL_DNS,
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
			Addr: dstAddr,
			Port: dstPort,
		},
		Dns: &aggregv1.DnsDetails{
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

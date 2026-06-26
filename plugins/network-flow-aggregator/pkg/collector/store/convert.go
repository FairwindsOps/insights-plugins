package store

import (
	aggregv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/aggregator/v1"
	insightsv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/insights/v1"
)

func cloneWorkloadRef(w *aggregv1.WorkloadRef) *insightsv1.WorkloadRef {
	if w == nil {
		return nil
	}
	return &insightsv1.WorkloadRef{
		Namespace: w.GetNamespace(),
		Pod:       w.GetPod(),
		Container: w.GetContainer(),
	}
}

func cloneEndpoint(e *aggregv1.Endpoint) *insightsv1.Endpoint {
	if e == nil {
		return nil
	}
	return &insightsv1.Endpoint{
		Addr: e.GetAddr(),
		Port: int32(e.GetPort()),
	}
}

func cloneDnsDetails(d *aggregv1.DnsDetails) *insightsv1.DnsDetails {
	if d == nil {
		return nil
	}
	return &insightsv1.DnsDetails{
		Name:      d.GetName(),
		Qtype:     d.GetQtype(),
		Rcode:     d.GetRcode(),
		Addresses: append([]string(nil), d.GetAddresses()...),
		QueryId:   int32(d.GetQueryId()),
	}
}

package store

import (
	aggregv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/aggregator/v1"
	networkflowv1 "github.com/fairwindsops/fairwinds-insights/pkg/networkflow/v1"
)

func cloneWorkloadRef(w *aggregv1.WorkloadRef) *networkflowv1.WorkloadRef {
	if w == nil {
		return nil
	}
	return &networkflowv1.WorkloadRef{
		Namespace: w.GetNamespace(),
		Pod:       w.GetPod(),
		Container: w.GetContainer(),
	}
}

func cloneEndpoint(e *aggregv1.Endpoint) *networkflowv1.Endpoint {
	if e == nil {
		return nil
	}
	return &networkflowv1.Endpoint{
		Addr: e.GetAddr(),
		Port: e.GetPort(),
	}
}

func cloneDnsDetails(d *aggregv1.DnsDetails) *networkflowv1.DnsDetails {
	if d == nil {
		return nil
	}
	return &networkflowv1.DnsDetails{
		Name:      d.GetName(),
		Qtype:     d.GetQtype(),
		Rcode:     d.GetRcode(),
		Addresses: append([]string(nil), d.GetAddresses()...),
		QueryId:   d.GetQueryId(),
	}
}

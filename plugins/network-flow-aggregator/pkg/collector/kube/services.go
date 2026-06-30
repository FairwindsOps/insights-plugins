package kube

import (
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type servicePortKey struct {
	clusterIP string
	port      uint32
}

type serviceIndex map[servicePortKey]serviceRef

type serviceRef struct {
	Namespace string
	Name      string
}

func buildServiceIndex(services []*corev1.Service) serviceIndex {
	idx := make(serviceIndex)
	for _, svc := range services {
		if svc == nil || svc.Spec.ClusterIP == "" || svc.Spec.ClusterIP == corev1.ClusterIPNone {
			continue
		}
		for _, p := range svc.Spec.Ports {
			idx[servicePortKey{clusterIP: svc.Spec.ClusterIP, port: uint32(p.Port)}] = serviceRef{
				Namespace: svc.Namespace,
				Name:      svc.Name,
			}
		}
	}
	return idx
}

func (idx serviceIndex) lookup(clusterIP string, port uint32) (serviceRef, bool) {
	ref, ok := idx[servicePortKey{clusterIP: clusterIP, port: port}]
	return ref, ok
}

func buildDstIndex(services []*corev1.Service, slices []*discoveryv1.EndpointSlice) serviceIndex {
	idx := buildServiceIndex(services)
	mergeEndpointSliceIndex(idx, slices)
	augmentEndpointIndexWithTargetPorts(idx, services, slices)
	return idx
}

func mergeEndpointSliceIndex(idx serviceIndex, slices []*discoveryv1.EndpointSlice) {
	for _, slice := range slices {
		if slice == nil {
			continue
		}
		svcName := slice.Labels[discoveryv1.LabelServiceName]
		if svcName == "" {
			continue
		}
		ref := serviceRef{Namespace: slice.Namespace, Name: svcName}
		for _, ep := range slice.Endpoints {
			if ep.Conditions.Ready != nil && !*ep.Conditions.Ready {
				continue
			}
			for _, addr := range ep.Addresses {
				for _, p := range slice.Ports {
					if p.Port == nil {
						continue
					}
					idx[servicePortKey{clusterIP: addr, port: uint32(*p.Port)}] = ref
				}
			}
		}
	}
}

func serviceAddressesByRef(slices []*discoveryv1.EndpointSlice) map[serviceRef][]string {
	addrs := make(map[serviceRef][]string)
	for _, slice := range slices {
		if slice == nil {
			continue
		}
		svcName := slice.Labels[discoveryv1.LabelServiceName]
		if svcName == "" {
			continue
		}
		ref := serviceRef{Namespace: slice.Namespace, Name: svcName}
		for _, ep := range slice.Endpoints {
			if ep.Conditions.Ready != nil && !*ep.Conditions.Ready {
				continue
			}
			addrs[ref] = append(addrs[ref], ep.Addresses...)
		}
	}
	return addrs
}

func augmentEndpointIndexWithTargetPorts(idx serviceIndex, services []*corev1.Service, slices []*discoveryv1.EndpointSlice) {
	addrsByService := serviceAddressesByRef(slices)
	for _, svc := range services {
		if svc == nil {
			continue
		}
		ref := serviceRef{Namespace: svc.Namespace, Name: svc.Name}
		addrs := addrsByService[ref]
		if len(addrs) == 0 {
			continue
		}
		for _, sp := range svc.Spec.Ports {
			if sp.TargetPort.Type != intstr.Int {
				continue
			}
			targetPort := uint32(sp.TargetPort.IntVal)
			if targetPort == uint32(sp.Port) {
				continue
			}
			for _, addr := range addrs {
				idx[servicePortKey{clusterIP: addr, port: targetPort}] = ref
			}
		}
	}
}

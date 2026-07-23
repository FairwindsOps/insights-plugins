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

// EndpointEntry maps a ready endpoint address:port to its Service and backing Pod.
type EndpointEntry struct {
	ServiceNamespace string
	ServiceName      string
	PodNamespace     string
	PodName          string
}

// BackendIdentity is the workload (and pod) that received traffic for a Service.
type BackendIdentity struct {
	PodNamespace      string
	PodName           string
	WorkloadNamespace string
	WorkloadKind      string
	WorkloadName      string
	ServiceNamespace  string
	ServiceName       string
}

func (b BackendIdentity) MatchesService(namespace, name string) bool {
	return b.ServiceNamespace == namespace && b.ServiceName == name
}

func buildServiceIndex(services []*corev1.Service) serviceIndex {
	idx := make(serviceIndex)
	for _, svc := range services {
		if svc == nil {
			continue
		}
		ref := serviceRef{Namespace: svc.Namespace, Name: svc.Name}
		for _, addr := range collectServiceAddrs(svc) {
			for _, p := range svc.Spec.Ports {
				idx[servicePortKey{clusterIP: addr, port: uint32(p.Port)}] = ref
			}
		}
	}
	return idx
}

func (idx serviceIndex) lookup(clusterIP string, port uint32) (serviceRef, bool) {
	ref, ok := idx[servicePortKey{clusterIP: clusterIP, port: port}]
	return ref, ok
}

type clusterIPIndexEntry struct {
	ref       serviceRef
	ambiguous bool
}

// clusterIPIndex maps a unique service address (any port) to its Service.
type clusterIPIndex map[string]clusterIPIndexEntry

func buildClusterIPIndex(services []*corev1.Service) clusterIPIndex {
	idx := make(clusterIPIndex)
	for _, svc := range services {
		if svc == nil {
			continue
		}
		ref := serviceRef{Namespace: svc.Namespace, Name: svc.Name}
		for _, addr := range collectServiceAddrs(svc) {
			addClusterIPIndexEntry(idx, addr, ref)
		}
	}
	return idx
}

func addClusterIPIndexEntry(idx clusterIPIndex, addr string, ref serviceRef) {
	ent, ok := idx[addr]
	if !ok {
		idx[addr] = clusterIPIndexEntry{ref: ref}
		return
	}
	if ent.ambiguous || ent.ref == ref {
		return
	}
	idx[addr] = clusterIPIndexEntry{ambiguous: true}
}

func (idx clusterIPIndex) lookup(clusterIP string) (serviceRef, bool) {
	ent, ok := idx[clusterIP]
	if !ok || ent.ambiguous {
		return serviceRef{}, false
	}
	return ent.ref, true
}

// collectServiceAddrs returns ClusterIP(s), ExternalIPs, and LB ingress IPs.
// Headless ClusterIP/ClusterIPs ("None") are skipped; ExternalIPs and LB IPs are still included.
func collectServiceAddrs(svc *corev1.Service) []string {
	if svc == nil {
		return nil
	}
	var addrs []string
	seen := make(map[string]struct{})
	add := func(ip string) {
		if ip == "" || ip == corev1.ClusterIPNone {
			return
		}
		if _, ok := seen[ip]; ok {
			return
		}
		seen[ip] = struct{}{}
		addrs = append(addrs, ip)
	}

	if svc.Spec.ClusterIP != corev1.ClusterIPNone {
		add(svc.Spec.ClusterIP)
		for _, ip := range svc.Spec.ClusterIPs {
			add(ip)
		}
	}
	for _, ip := range svc.Spec.ExternalIPs {
		add(ip)
	}
	for _, ing := range svc.Status.LoadBalancer.Ingress {
		add(ing.IP)
	}
	return addrs
}

type endpointIPIndexEntry struct {
	ref       serviceRef
	ambiguous bool
}

// endpointIPIndex maps a unique ready EndpointSlice address (any port) to its Service.
type endpointIPIndex map[string]endpointIPIndexEntry

func buildEndpointIPIndex(slices []*discoveryv1.EndpointSlice) endpointIPIndex {
	idx := make(endpointIPIndex)
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
				ent, ok := idx[addr]
				if !ok {
					idx[addr] = endpointIPIndexEntry{ref: ref}
					continue
				}
				if ent.ambiguous || ent.ref == ref {
					continue
				}
				idx[addr] = endpointIPIndexEntry{ambiguous: true}
			}
		}
	}
	return idx
}

func (idx endpointIPIndex) lookup(addr string) (serviceRef, bool) {
	ent, ok := idx[addr]
	if !ok || ent.ambiguous {
		return serviceRef{}, false
	}
	return ent.ref, true
}

func buildDstIndex(services []*corev1.Service, slices []*discoveryv1.EndpointSlice) serviceIndex {
	idx := buildServiceIndex(services)
	mergeEndpointSliceIndex(idx, slices)
	augmentEndpointIndexWithTargetPorts(idx, services, slices)
	return idx
}

type endpointIndex map[servicePortKey]EndpointEntry

func buildEndpointIndex(services []*corev1.Service, slices []*discoveryv1.EndpointSlice) endpointIndex {
	idx := make(endpointIndex)
	mergeEndpointSlicePodIndex(idx, slices)
	augmentEndpointPodIndexWithTargetPorts(idx, services, slices)
	return idx
}

func (idx endpointIndex) lookup(addr string, port uint32) (EndpointEntry, bool) {
	entry, ok := idx[servicePortKey{clusterIP: addr, port: port}]
	return entry, ok
}

func mergeEndpointSlicePodIndex(idx endpointIndex, slices []*discoveryv1.EndpointSlice) {
	for _, slice := range slices {
		if slice == nil {
			continue
		}
		svcName := slice.Labels[discoveryv1.LabelServiceName]
		if svcName == "" {
			continue
		}
		for _, ep := range slice.Endpoints {
			if ep.Conditions.Ready != nil && !*ep.Conditions.Ready {
				continue
			}
			podNamespace, podName := endpointPodIdentity(slice.Namespace, ep)
			for _, addr := range ep.Addresses {
				for _, p := range slice.Ports {
					if p.Port == nil {
						continue
					}
					idx[servicePortKey{clusterIP: addr, port: uint32(*p.Port)}] = EndpointEntry{
						ServiceNamespace: slice.Namespace,
						ServiceName:      svcName,
						PodNamespace:     podNamespace,
						PodName:          podName,
					}
				}
			}
		}
	}
}

func augmentEndpointPodIndexWithTargetPorts(idx endpointIndex, services []*corev1.Service, slices []*discoveryv1.EndpointSlice) {
	addrsByService := serviceAddressesByRef(slices)
	podByAddr := endpointPodsByAddress(slices)
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
				pod := podByAddr[addr]
				idx[servicePortKey{clusterIP: addr, port: targetPort}] = EndpointEntry{
					ServiceNamespace: svc.Namespace,
					ServiceName:      svc.Name,
					PodNamespace:     pod.Namespace,
					PodName:          pod.Name,
				}
			}
		}
	}
}

func endpointPodIdentity(sliceNamespace string, ep discoveryv1.Endpoint) (namespace, name string) {
	if ep.TargetRef != nil && ep.TargetRef.Kind == "Pod" {
		namespace = ep.TargetRef.Namespace
		if namespace == "" {
			namespace = sliceNamespace
		}
		return namespace, ep.TargetRef.Name
	}
	return "", ""
}

func endpointPodsByAddress(slices []*discoveryv1.EndpointSlice) map[string]struct{ Namespace, Name string } {
	out := make(map[string]struct{ Namespace, Name string })
	for _, slice := range slices {
		if slice == nil {
			continue
		}
		for _, ep := range slice.Endpoints {
			if ep.Conditions.Ready != nil && !*ep.Conditions.Ready {
				continue
			}
			podNamespace, podName := endpointPodIdentity(slice.Namespace, ep)
			for _, addr := range ep.Addresses {
				out[addr] = struct{ Namespace, Name string }{Namespace: podNamespace, Name: podName}
			}
		}
	}
	return out
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

package kube

import corev1 "k8s.io/api/core/v1"

type podRef struct {
	Namespace string
	Name      string
}

type podIPIndexEntry struct {
	ref       podRef
	ambiguous bool
}

type podIPIndex map[string]podIPIndexEntry

func buildPodIPIndex(pods []*corev1.Pod) podIPIndex {
	idx := make(podIPIndex)
	for _, pod := range pods {
		if pod == nil || !indexablePod(pod) {
			continue
		}
		ref := podRef{Namespace: pod.Namespace, Name: pod.Name}
		for _, ip := range podIPs(pod) {
			if ip == "" {
				continue
			}
			ent, ok := idx[ip]
			if !ok {
				idx[ip] = podIPIndexEntry{ref: ref}
				continue
			}
			if ent.ambiguous || ent.ref == ref {
				continue
			}
			idx[ip] = podIPIndexEntry{ambiguous: true}
		}
	}
	return idx
}

func (idx podIPIndex) lookup(addr string) (podRef, bool) {
	ent, ok := idx[addr]
	if !ok || ent.ambiguous {
		return podRef{}, false
	}
	return ent.ref, true
}

func indexablePod(pod *corev1.Pod) bool {
	if pod.Spec.HostNetwork {
		return false
	}
	switch pod.Status.Phase {
	case corev1.PodSucceeded, corev1.PodFailed:
		return false
	}
	return true
}

func podIPs(pod *corev1.Pod) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(ip string) {
		if ip == "" {
			return
		}
		if _, ok := seen[ip]; ok {
			return
		}
		seen[ip] = struct{}{}
		out = append(out, ip)
	}
	add(pod.Status.PodIP)
	for _, pip := range pod.Status.PodIPs {
		add(pip.IP)
	}
	return out
}

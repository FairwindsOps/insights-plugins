package kube

import corev1 "k8s.io/api/core/v1"

type nodeIPIndexEntry struct {
	name      string
	ambiguous bool
}

type nodeIPIndex map[string]nodeIPIndexEntry

func buildNodeIPIndex(nodes []*corev1.Node) nodeIPIndex {
	idx := make(nodeIPIndex)
	for _, node := range nodes {
		if node == nil || node.Name == "" {
			continue
		}
		for _, addr := range node.Status.Addresses {
			switch addr.Type {
			case corev1.NodeInternalIP, corev1.NodeExternalIP:
			default:
				continue
			}
			ip := addr.Address
			if ip == "" {
				continue
			}
			ent, ok := idx[ip]
			if !ok {
				idx[ip] = nodeIPIndexEntry{name: node.Name}
				continue
			}
			if ent.ambiguous || ent.name == node.Name {
				continue
			}
			idx[ip] = nodeIPIndexEntry{ambiguous: true}
		}
	}
	return idx
}

func (idx nodeIPIndex) lookup(addr string) (string, bool) {
	ent, ok := idx[addr]
	if !ok || ent.ambiguous {
		return "", false
	}
	return ent.name, true
}

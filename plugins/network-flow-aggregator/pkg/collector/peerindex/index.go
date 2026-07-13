package peerindex

import (
	"sync"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/kube"
)

type peerKey struct {
	addr string
	port uint32
}

type entryWithExpiry struct {
	entry   kube.BackendIdentity
	expires time.Time
}

// Index maps client (src_addr, src_port) to the backend pod/workload that accepted the connection.
type Index struct {
	mu      sync.Mutex
	entries map[peerKey]entryWithExpiry
	ttl     time.Duration
}

func New(ttl time.Duration) *Index {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Index{
		entries: make(map[peerKey]entryWithExpiry),
		ttl:     ttl,
	}
}

func (idx *Index) Put(clientAddr string, clientPort uint32, backend kube.BackendIdentity, now time.Time) {
	if clientAddr == "" || clientPort == 0 {
		return
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.pruneLocked(now)
	idx.entries[peerKey{addr: clientAddr, port: clientPort}] = entryWithExpiry{
		entry:   backend,
		expires: now.Add(idx.ttl),
	}
}

func (idx *Index) Lookup(clientAddr string, clientPort uint32) (kube.BackendIdentity, bool) {
	if clientAddr == "" || clientPort == 0 {
		return kube.BackendIdentity{}, false
	}
	now := time.Now()
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.pruneLocked(now)
	item, ok := idx.entries[peerKey{addr: clientAddr, port: clientPort}]
	if !ok || now.After(item.expires) {
		return kube.BackendIdentity{}, false
	}
	return item.entry, true
}

func (idx *Index) pruneLocked(now time.Time) {
	for key, item := range idx.entries {
		if now.After(item.expires) {
			delete(idx.entries, key)
		}
	}
}

package agent

import "sync"

type connKey struct {
	Namespace string
	Pod       string
	Container string
	SrcAddr   string
	SrcPort   uint32
	DstAddr   string
	DstPort   uint32
	PID       uint32
}

type byteSnapshot struct {
	sent     uint64
	received uint64
}

type ByteDeltaTracker struct {
	mu   sync.Mutex
	last map[connKey]byteSnapshot
}

func NewByteDeltaTracker() *ByteDeltaTracker {
	return &ByteDeltaTracker{last: make(map[connKey]byteSnapshot)}
}

func (t *ByteDeltaTracker) Delta(key connKey, sent, received uint64) (deltaSent, deltaReceived uint64, ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	prev, seen := t.last[key]
	t.last[key] = byteSnapshot{sent: sent, received: received}
	if !seen {
		return 0, 0, false
	}

	deltaSent = diffCumulative(prev.sent, sent)
	deltaReceived = diffCumulative(prev.received, received)
	return deltaSent, deltaReceived, deltaSent > 0 || deltaReceived > 0
}

func diffCumulative(prev, current uint64) uint64 {
	if current >= prev {
		return current - prev
	}
	return current
}

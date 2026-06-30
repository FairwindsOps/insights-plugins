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

type connState struct {
	fields          TCPFields
	lastSent        uint64
	lastReceived    uint64
	emittedSent     uint64
	emittedReceived uint64
	seen            bool
}

// ByteDelta is a traffic byte increment for one connection.
type ByteDelta struct {
	Fields   TCPFields
	Sent     uint64
	Received uint64
}

type ByteDeltaTracker struct {
	mu   sync.Mutex
	last map[connKey]*connState
}

func NewByteDeltaTracker() *ByteDeltaTracker {
	return &ByteDeltaTracker{last: make(map[connKey]*connState)}
}

func (t *ByteDeltaTracker) BeginPoll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, st := range t.last {
		st.seen = false
	}
}

func (t *ByteDeltaTracker) Observe(fields TCPFields) (delta ByteDelta, ok bool) {
	key := connKeyFromFields(fields)

	t.mu.Lock()
	defer t.mu.Unlock()

	st, exists := t.last[key]
	if !exists {
		t.last[key] = &connState{
			fields:       fields,
			lastSent:     fields.BytesSent,
			lastReceived: fields.BytesReceived,
			seen:         true,
		}
		return ByteDelta{}, false
	}

	st.seen = true
	st.fields = fields
	deltaSent := diffCumulative(st.lastSent, fields.BytesSent)
	deltaReceived := diffCumulative(st.lastReceived, fields.BytesReceived)
	st.lastSent = fields.BytesSent
	st.lastReceived = fields.BytesReceived

	if deltaSent == 0 && deltaReceived == 0 {
		return ByteDelta{}, false
	}

	st.emittedSent += deltaSent
	st.emittedReceived += deltaReceived
	return ByteDelta{Fields: fields, Sent: deltaSent, Received: deltaReceived}, true
}

func (t *ByteDeltaTracker) EndPoll() []ByteDelta {
	t.mu.Lock()
	defer t.mu.Unlock()

	var out []ByteDelta
	for key, st := range t.last {
		if st.seen {
			continue
		}
		remainingSent := remainingBytes(st.emittedSent, st.lastSent)
		remainingReceived := remainingBytes(st.emittedReceived, st.lastReceived)
		delete(t.last, key)
		if remainingSent > 0 || remainingReceived > 0 {
			out = append(out, ByteDelta{
				Fields:   st.fields,
				Sent:     remainingSent,
				Received: remainingReceived,
			})
		}
	}
	return out
}

// Delta reports byte increments between consecutive observations for one key.
// Prefer BeginPoll/Observe/EndPoll for polling gadgets such as top_tcp.
func (t *ByteDeltaTracker) Delta(key connKey, sent, received uint64) (deltaSent, deltaReceived uint64, ok bool) {
	delta, ok := t.Observe(TCPFields{
		Namespace:     key.Namespace,
		Pod:           key.Pod,
		Container:     key.Container,
		SrcAddr:       key.SrcAddr,
		SrcPort:       key.SrcPort,
		DstAddr:       key.DstAddr,
		DstPort:       key.DstPort,
		PID:           key.PID,
		BytesSent:     sent,
		BytesReceived: received,
	})
	return delta.Sent, delta.Received, ok
}

func connKeyFromFields(fields TCPFields) connKey {
	return connKey{
		Namespace: fields.Namespace,
		Pod:       fields.Pod,
		Container: fields.Container,
		SrcAddr:   fields.SrcAddr,
		SrcPort:   fields.SrcPort,
		DstAddr:   fields.DstAddr,
		DstPort:   fields.DstPort,
		PID:       fields.PID,
	}
}

func remainingBytes(emitted, last uint64) uint64 {
	if last >= emitted {
		return last - emitted
	}
	return last
}

func diffCumulative(prev, current uint64) uint64 {
	if current >= prev {
		return current - prev
	}
	return current
}

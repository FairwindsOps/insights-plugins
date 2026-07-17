package store

import (
	"sync"
	"time"

	aggregv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/aggregator/v1"
	insightsv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/insights/v1"
)

type Enrichment struct {
	SrcNamespace    string
	SrcWorkloadKind string
	SrcWorkloadName string
	DstNamespace    string
	DstKind         string
	DstName         string
	BackendWorkloadNamespace string
	BackendWorkloadKind      string
	BackendWorkloadName      string
	BackendPodNamespace      string
	BackendPodName           string
}

type ListOpts struct {
	Since           int64
	Limit           int
	Offset          int
	Namespace       string
	EventKind       insightsv1.FlowEventKind
	SrcWorkloadKind string
	DstKind         string
}

type Store struct {
	mu             sync.RWMutex
	events         []*insightsv1.EnrichedFlowEvent
	maxEvents      int
	maxAge         time.Duration
	sendCursor     int
	droppedUnsent  int64
	lastDropReason string
}

func NewStore(maxEvents int, maxAge time.Duration) *Store {
	if maxEvents <= 0 {
		maxEvents = 100_000
	}
	if maxAge <= 0 {
		maxAge = 15 * time.Minute
	}
	return &Store{
		events:    make([]*insightsv1.EnrichedFlowEvent, 0, 1024),
		maxEvents: maxEvents,
		maxAge:    maxAge,
	}
}

func (s *Store) MaxAge() time.Duration {
	return s.maxAge
}

func (s *Store) AppendBatch(batch *aggregv1.FlowEventBatch, enrich func(*aggregv1.FlowEvent) Enrichment) (int64, []*insightsv1.EnrichedFlowEvent) {
	if batch == nil {
		return 0, nil
	}

	// Enrich outside the store lock: enrichment may hit the API / informers and must
	// not block upstream send or concurrent AppendBatch callers.
	events := batch.GetEvents()
	rows := make([]*insightsv1.EnrichedFlowEvent, 0, len(events))
	for _, event := range events {
		if event == nil || !isAcceptableEvent(event) {
			continue
		}

		var enrichment Enrichment
		if enrich != nil {
			enrichment = enrich(event)
		} else {
			enrichment = Enrichment{
				SrcNamespace:    event.GetSrc().GetNamespace(),
				SrcWorkloadKind: "Pod",
				SrcWorkloadName: event.GetSrc().GetPod(),
			}
		}
		rows = append(rows, enrichedFromEvent(batch.GetNodeName(), batch.GetAgentId(), event, enrichment))
	}
	if len(rows) == 0 {
		return 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = append(s.events, rows...)
	s.pruneLocked()
	s.enforceMaxLocked()
	return int64(len(rows)), rows
}

func enrichedFromEvent(nodeName, agentID string, event *aggregv1.FlowEvent, enrich Enrichment) *insightsv1.EnrichedFlowEvent {
	out := &insightsv1.EnrichedFlowEvent{
		NodeName:          nodeName,
		AgentId:           agentID,
		EventKind:         insightsv1.FlowEventKind(event.GetEventKind()),
		Protocol:          insightsv1.Protocol(event.GetProtocol()),
		TimestampUnixNano: event.GetTimestampUnixNano(),
		Src:               cloneWorkloadRef(event.GetSrc()),
		SrcEndpoint:       cloneEndpoint(event.GetSrcEndpoint()),
		Dst:               cloneEndpoint(event.GetDst()),
		BytesSent:         int64(event.GetBytesSent()),
		BytesReceived:     int64(event.GetBytesReceived()),
		Dns:               cloneDnsDetails(event.GetDns()),
	}
	if enrich.SrcNamespace != "" || enrich.SrcWorkloadKind != "" || enrich.SrcWorkloadName != "" {
		out.SrcWorkload = &insightsv1.KubernetesRef{
			Namespace: enrich.SrcNamespace,
			Kind:      enrich.SrcWorkloadKind,
			Name:      enrich.SrcWorkloadName,
		}
	}
	if enrich.DstKind != "" || enrich.DstName != "" {
		out.DstRef = &insightsv1.KubernetesRef{
			Namespace: enrich.DstNamespace,
			Kind:      enrich.DstKind,
			Name:      enrich.DstName,
		}
	}
	if enrich.BackendWorkloadKind != "" || enrich.BackendWorkloadName != "" {
		out.BackendWorkload = &insightsv1.KubernetesRef{
			Namespace: enrich.BackendWorkloadNamespace,
			Kind:      enrich.BackendWorkloadKind,
			Name:      enrich.BackendWorkloadName,
		}
	}
	if enrich.BackendPodName != "" {
		out.BackendPod = &insightsv1.WorkloadRef{
			Namespace: enrich.BackendPodNamespace,
			Pod:       enrich.BackendPodName,
		}
	}
	return out
}

func isAcceptableEvent(event *aggregv1.FlowEvent) bool {
	if event.GetSrc().GetPod() == "" {
		return false
	}
	if event.GetProtocol() == aggregv1.Protocol_PROTOCOL_DNS {
		return event.GetDns().GetName() != ""
	}
	return event.GetDst().GetAddr() != ""
}

func (s *Store) pruneLocked() {
	if s.maxAge <= 0 || len(s.events) == 0 {
		return
	}
	cutoff := time.Now().Add(-s.maxAge).UnixNano()
	start := 0
	for start < len(s.events) && s.events[start].GetTimestampUnixNano() < cutoff {
		start++
	}
	if start > 0 {
		s.events = append([]*insightsv1.EnrichedFlowEvent(nil), s.events[start:]...)
		s.adjustCursorOnDrop(start, "max_age")
	}
}

func (s *Store) enforceMaxLocked() {
	overflow := len(s.events) - s.maxEvents
	if overflow <= 0 {
		return
	}
	s.events = append([]*insightsv1.EnrichedFlowEvent(nil), s.events[overflow:]...)
	s.adjustCursorOnDrop(overflow, "max_events")
}

func (s *Store) adjustCursorOnDrop(droppedFromFront int, reason string) {
	if droppedFromFront <= 0 {
		return
	}
	var lostUnsent int
	if s.sendCursor >= droppedFromFront {
		s.sendCursor -= droppedFromFront
	} else {
		lostUnsent = droppedFromFront - s.sendCursor
		s.sendCursor = 0
	}
	if lostUnsent > 0 {
		s.droppedUnsent += int64(lostUnsent)
		s.lastDropReason = reason
	}
}

func (s *Store) UnsentCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.sendCursor >= len(s.events) {
		return 0
	}
	return len(s.events) - s.sendCursor
}

// OldestUnsentAge returns how long the oldest unsent event has been buffered.
// Returns 0 when there is no unsent event.
func (s *Store) OldestUnsentAge() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := s.sendCursor; i < len(s.events); i++ {
		e := s.events[i]
		if e == nil {
			continue
		}
		ts := e.GetTimestampUnixNano()
		if ts <= 0 {
			return 0
		}
		age := time.Since(time.Unix(0, ts))
		if age < 0 {
			return 0
		}
		return age
	}
	return 0
}

func (s *Store) SendCursor() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sendCursor
}

func (s *Store) PeekUnsentBatch(maxEvents int) (nodeName, agentID string, events []*insightsv1.EnrichedFlowEvent, ok bool) {
	if maxEvents <= 0 {
		maxEvents = 100
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	i := s.sendCursor
	for i < len(s.events) && s.events[i] == nil {
		i++
	}
	if i >= len(s.events) {
		return "", "", nil, false
	}

	nodeName = s.events[i].GetNodeName()
	agentID = s.events[i].GetAgentId()
	events = make([]*insightsv1.EnrichedFlowEvent, 0, maxEvents)

	for ; i < len(s.events) && len(events) < maxEvents; i++ {
		e := s.events[i]
		if e == nil {
			continue
		}
		if e.GetNodeName() != nodeName || e.GetAgentId() != agentID {
			break
		}
		events = append(events, e)
	}
	if len(events) == 0 {
		return "", "", nil, false
	}
	return nodeName, agentID, events, true
}

func (s *Store) AdvanceSendCursor(n int) {
	if n <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendCursor += n
	if s.sendCursor > len(s.events) {
		s.sendCursor = len(s.events)
	}
}

func (s *Store) TakeDroppedUnsent() (count int64, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count = s.droppedUnsent
	reason = s.lastDropReason
	s.droppedUnsent = 0
	s.lastDropReason = ""
	return count, reason
}

func (s *Store) ListEvents(opts ListOpts) []*insightsv1.EnrichedFlowEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	limit := opts.Limit
	if limit <= 0 {
		limit = len(s.events)
	}

	out := make([]*insightsv1.EnrichedFlowEvent, 0, limit)
	skipped := 0
	for _, event := range s.events {
		if event == nil {
			continue
		}
		if opts.Since > 0 && event.GetTimestampUnixNano() <= opts.Since {
			continue
		}
		if opts.Namespace != "" && event.GetSrc().GetNamespace() != opts.Namespace && event.GetDstRef().GetNamespace() != opts.Namespace {
			continue
		}
		if opts.EventKind != insightsv1.FlowEventKind_FLOW_EVENT_KIND_UNSPECIFIED && event.GetEventKind() != opts.EventKind {
			continue
		}
		if opts.SrcWorkloadKind != "" && event.GetSrcWorkload().GetKind() != opts.SrcWorkloadKind {
			continue
		}
		if opts.DstKind != "" && event.GetDstRef().GetKind() != opts.DstKind {
			continue
		}
		if skipped < opts.Offset {
			skipped++
			continue
		}
		out = append(out, event)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events)
}

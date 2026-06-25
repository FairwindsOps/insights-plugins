package store

import (
	"sync"
	"time"

	aggregv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/aggregator/v1"
	networkflowv1 "github.com/fairwindsops/fairwinds-insights/pkg/networkflow/v1"
)

type Enrichment struct {
	SrcNamespace    string
	SrcWorkloadKind string
	SrcWorkloadName string
	DstNamespace    string
	DstKind         string
	DstName         string
}

type ListOpts struct {
	Since           int64
	Limit           int
	Offset          int
	Namespace       string
	EventKind       networkflowv1.FlowEventKind
	SrcWorkloadKind string
	DstKind         string
}

type Store struct {
	mu             sync.RWMutex
	events         []*networkflowv1.EnrichedFlowEvent
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
		events:    make([]*networkflowv1.EnrichedFlowEvent, 0, 1024),
		maxEvents: maxEvents,
		maxAge:    maxAge,
	}
}

func (s *Store) MaxAge() time.Duration {
	return s.maxAge
}

func (s *Store) AppendBatch(batch *aggregv1.FlowEventBatch, enrich func(*aggregv1.FlowEvent) Enrichment) (int64, []*networkflowv1.EnrichedFlowEvent) {
	if batch == nil {
		return 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var accepted int64
	enriched := make([]*networkflowv1.EnrichedFlowEvent, 0, len(batch.GetEvents()))
	for _, event := range batch.GetEvents() {
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

		row := enrichedFromEvent(batch.GetNodeName(), batch.GetAgentId(), event, enrichment)
		s.events = append(s.events, row)
		enriched = append(enriched, row)
		accepted++
	}

	s.pruneLocked()
	s.enforceMaxLocked()
	return accepted, enriched
}

func enrichedFromEvent(nodeName, agentID string, event *aggregv1.FlowEvent, enrich Enrichment) *networkflowv1.EnrichedFlowEvent {
	out := &networkflowv1.EnrichedFlowEvent{
		NodeName:          nodeName,
		AgentId:           agentID,
		EventKind:         networkflowv1.FlowEventKind(event.GetEventKind()),
		Protocol:          networkflowv1.Protocol(event.GetProtocol()),
		TimestampUnixNano: event.GetTimestampUnixNano(),
		Src:               cloneWorkloadRef(event.GetSrc()),
		SrcEndpoint:       cloneEndpoint(event.GetSrcEndpoint()),
		Dst:               cloneEndpoint(event.GetDst()),
		BytesSent:         int64(event.GetBytesSent()),
		BytesReceived:     int64(event.GetBytesReceived()),
		Dns:               cloneDnsDetails(event.GetDns()),
	}
	if enrich.SrcNamespace != "" || enrich.SrcWorkloadKind != "" || enrich.SrcWorkloadName != "" {
		out.SrcWorkload = &networkflowv1.KubernetesRef{
			Namespace: enrich.SrcNamespace,
			Kind:      enrich.SrcWorkloadKind,
			Name:      enrich.SrcWorkloadName,
		}
	}
	if enrich.DstKind != "" || enrich.DstName != "" {
		out.DstRef = &networkflowv1.KubernetesRef{
			Namespace: enrich.DstNamespace,
			Kind:      enrich.DstKind,
			Name:      enrich.DstName,
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
		s.events = append([]*networkflowv1.EnrichedFlowEvent(nil), s.events[start:]...)
		s.adjustCursorOnDrop(start, "max_age")
	}
}

func (s *Store) enforceMaxLocked() {
	overflow := len(s.events) - s.maxEvents
	if overflow <= 0 {
		return
	}
	s.events = append([]*networkflowv1.EnrichedFlowEvent(nil), s.events[overflow:]...)
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

func (s *Store) SendCursor() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sendCursor
}

func (s *Store) PeekUnsentBatch(maxEvents int) (nodeName, agentID string, events []*networkflowv1.EnrichedFlowEvent, ok bool) {
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
	events = make([]*networkflowv1.EnrichedFlowEvent, 0, maxEvents)

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

func (s *Store) ListEvents(opts ListOpts) []*networkflowv1.EnrichedFlowEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	limit := opts.Limit
	if limit <= 0 {
		limit = len(s.events)
	}

	out := make([]*networkflowv1.EnrichedFlowEvent, 0, limit)
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
		if opts.EventKind != networkflowv1.FlowEventKind_FLOW_EVENT_KIND_UNSPECIFIED && event.GetEventKind() != opts.EventKind {
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

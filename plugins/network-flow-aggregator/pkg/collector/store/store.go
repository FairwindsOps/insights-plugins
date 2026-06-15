package store

import (
	"sync"
	"time"

	flowv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow/pkg/flow/v1"
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
	Since            int64
	Limit            int
	Offset           int
	Namespace        string
	EventKind        flowv1.FlowEventKind
	SrcWorkloadKind  string
	DstKind          string
}

type Store struct {
	mu        sync.RWMutex
	events    []*flowv1.EnrichedFlowEvent
	maxEvents int
	maxAge    time.Duration
}

func NewStore(maxEvents int, maxAge time.Duration) *Store {
	if maxEvents <= 0 {
		maxEvents = 100_000
	}
	if maxAge <= 0 {
		maxAge = 15 * time.Minute
	}
	return &Store{
		events:    make([]*flowv1.EnrichedFlowEvent, 0, 1024),
		maxEvents: maxEvents,
		maxAge:    maxAge,
	}
}

func (s *Store) MaxAge() time.Duration {
	return s.maxAge
}

func (s *Store) AppendBatch(batch *flowv1.FlowEventBatch, enrich func(*flowv1.FlowEvent) Enrichment) (int64, []*flowv1.EnrichedFlowEvent) {
	if batch == nil {
		return 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var accepted int64
	enriched := make([]*flowv1.EnrichedFlowEvent, 0, len(batch.GetEvents()))
	for _, event := range batch.GetEvents() {
		if event == nil || event.GetSrc().GetPod() == "" || event.GetDst().GetAddr() == "" {
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

func enrichedFromEvent(nodeName, agentID string, event *flowv1.FlowEvent, enrich Enrichment) *flowv1.EnrichedFlowEvent {
	out := &flowv1.EnrichedFlowEvent{
		NodeName:          nodeName,
		AgentId:           agentID,
		EventKind:         event.GetEventKind(),
		Protocol:          event.GetProtocol(),
		TimestampUnixNano: event.GetTimestampUnixNano(),
		Src:               event.GetSrc(),
		SrcEndpoint:       event.GetSrcEndpoint(),
		Dst:               event.GetDst(),
		BytesSent:         event.GetBytesSent(),
		BytesReceived:     event.GetBytesReceived(),
	}
	if enrich.SrcNamespace != "" || enrich.SrcWorkloadKind != "" || enrich.SrcWorkloadName != "" {
		out.SrcWorkload = &flowv1.KubernetesRef{
			Namespace: enrich.SrcNamespace,
			Kind:      enrich.SrcWorkloadKind,
			Name:      enrich.SrcWorkloadName,
		}
	}
	if enrich.DstKind != "" || enrich.DstName != "" {
		out.DstRef = &flowv1.KubernetesRef{
			Namespace: enrich.DstNamespace,
			Kind:      enrich.DstKind,
			Name:      enrich.DstName,
		}
	}
	return out
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
		s.events = append([]*flowv1.EnrichedFlowEvent(nil), s.events[start:]...)
	}
}

func (s *Store) enforceMaxLocked() {
	overflow := len(s.events) - s.maxEvents
	if overflow <= 0 {
		return
	}
	s.events = append([]*flowv1.EnrichedFlowEvent(nil), s.events[overflow:]...)
}

func (s *Store) ListEvents(opts ListOpts) []*flowv1.EnrichedFlowEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	limit := opts.Limit
	if limit <= 0 {
		limit = len(s.events)
	}

	out := make([]*flowv1.EnrichedFlowEvent, 0, limit)
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
		if opts.EventKind != flowv1.FlowEventKind_FLOW_EVENT_KIND_UNSPECIFIED && event.GetEventKind() != opts.EventKind {
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

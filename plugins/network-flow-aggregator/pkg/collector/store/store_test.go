package store

import (
	"testing"
	"time"

	aggregv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/aggregator/v1"
	networkflowv1 "github.com/fairwindsops/fairwinds-insights/pkg/networkflow/v1"
)

func TestAppendBatchDoesNotMerge(t *testing.T) {
	st := NewStore(1000, time.Hour)
	now := time.Now().UnixNano()
	batch := &aggregv1.FlowEventBatch{
		NodeName: "node-a",
		AgentId:  "agent-a",
		Events: []*aggregv1.FlowEvent{
			sampleEvent(now, 0, 0),
			sampleEvent(now+1, 0, 0),
		},
	}

	enrich := func(_ *aggregv1.FlowEvent) Enrichment {
		return Enrichment{
			SrcNamespace:    "default",
			SrcWorkloadKind: "Deployment",
			SrcWorkloadName: "payments",
		}
	}

	if got, _ := st.AppendBatch(batch, enrich); got != 2 {
		t.Fatalf("accepted = %d", got)
	}
	if st.Count() != 2 {
		t.Fatalf("events = %d", st.Count())
	}
}

func TestAppendBatchPreservesBytes(t *testing.T) {
	st := NewStore(1000, time.Hour)
	now := time.Now().UnixNano()
	batch := &aggregv1.FlowEventBatch{
		Events: []*aggregv1.FlowEvent{
			sampleEvent(now, 100, 200),
			sampleEvent(now+1, 50, 60),
		},
	}

	if got, _ := st.AppendBatch(batch, nil); got != 2 {
		t.Fatalf("accepted = %d", got)
	}
	events := st.ListEvents(ListOpts{})
	if events[0].GetBytesSent() != 100 || events[0].GetBytesReceived() != 200 {
		t.Fatalf("first bytes = sent:%d recv:%d", events[0].GetBytesSent(), events[0].GetBytesReceived())
	}
	if events[1].GetBytesSent() != 50 || events[1].GetBytesReceived() != 60 {
		t.Fatalf("second bytes = sent:%d recv:%d", events[1].GetBytesSent(), events[1].GetBytesReceived())
	}
}

func TestAppendBatchEnrichment(t *testing.T) {
	st := NewStore(1000, time.Hour)
	batch := &aggregv1.FlowEventBatch{
		Events: []*aggregv1.FlowEvent{
			{
				EventKind:         aggregv1.FlowEventKind_FLOW_EVENT_KIND_CONNECT,
				Protocol:          aggregv1.Protocol_PROTOCOL_TCP,
				TimestampUnixNano: time.Now().UnixNano(),
				Src:               &aggregv1.WorkloadRef{Namespace: "prod", Pod: "payments-abc"},
				Dst:               &aggregv1.Endpoint{Addr: "10.96.0.10", Port: 5432},
			},
		},
	}

	enrich := func(_ *aggregv1.FlowEvent) Enrichment {
		return Enrichment{
			SrcNamespace:    "prod",
			SrcWorkloadKind: "Deployment",
			SrcWorkloadName: "payments",
			DstNamespace:    "prod",
			DstKind:         "Service",
			DstName:         "postgres",
		}
	}

	if got, _ := st.AppendBatch(batch, enrich); got != 1 {
		t.Fatalf("accepted = %d", got)
	}
	events := st.ListEvents(ListOpts{})
	if events[0].GetSrcWorkload().GetKind() != "Deployment" {
		t.Fatalf("src workload = %#v", events[0].GetSrcWorkload())
	}
	if events[0].GetDstRef().GetKind() != "Service" || events[0].GetDstRef().GetName() != "postgres" {
		t.Fatalf("dst ref = %#v", events[0].GetDstRef())
	}
}

func TestStoreEnforcesMaxEvents(t *testing.T) {
	st := NewStore(2, time.Hour)
	now := time.Now().UnixNano()
	batch := &aggregv1.FlowEventBatch{
		Events: []*aggregv1.FlowEvent{
			sampleEvent(now, 0, 0),
			sampleEvent(now+1, 0, 0),
			sampleEvent(now+2, 0, 0),
		},
	}
	st.AppendBatch(batch, nil)
	events := st.ListEvents(ListOpts{})
	if len(events) != 2 {
		t.Fatalf("events = %d", len(events))
	}
	if events[0].GetTimestampUnixNano() != now+1 || events[1].GetTimestampUnixNano() != now+2 {
		t.Fatalf("unexpected retained events: %+v", events)
	}
}

func TestAdvanceSendCursor(t *testing.T) {
	st := NewStore(100, time.Hour)
	now := time.Now().UnixNano()
	st.AppendBatch(&aggregv1.FlowEventBatch{
		NodeName: "node-a",
		AgentId:  "agent-a",
		Events: []*aggregv1.FlowEvent{
			sampleEvent(now, 0, 0),
			sampleEvent(now+1, 0, 0),
		},
	}, nil)

	if st.UnsentCount() != 2 {
		t.Fatalf("unsent = %d", st.UnsentCount())
	}
	st.AdvanceSendCursor(1)
	if st.UnsentCount() != 1 {
		t.Fatalf("unsent after advance = %d", st.UnsentCount())
	}
	if st.SendCursor() != 1 {
		t.Fatalf("cursor = %d", st.SendCursor())
	}
}

func TestEnforceMaxDropsUnsentEvents(t *testing.T) {
	st := NewStore(2, time.Hour)
	now := time.Now().UnixNano()
	st.AppendBatch(&aggregv1.FlowEventBatch{
		NodeName: "node-a",
		AgentId:  "agent-a",
		Events: []*aggregv1.FlowEvent{
			sampleEvent(now, 0, 0),
			sampleEvent(now+1, 0, 0),
			sampleEvent(now+2, 0, 0),
		},
	}, nil)

	if st.UnsentCount() != 2 {
		t.Fatalf("unsent = %d, want 2", st.UnsentCount())
	}
	if st.SendCursor() != 0 {
		t.Fatalf("cursor = %d, want 0", st.SendCursor())
	}
	dropped, reason := st.TakeDroppedUnsent()
	if dropped != 1 {
		t.Fatalf("dropped unsent = %d, want 1", dropped)
	}
	if reason != "max_events" {
		t.Fatalf("drop reason = %q", reason)
	}
}

func TestPeekUnsentBatchGroupsByAgent(t *testing.T) {
	st := NewStore(100, time.Hour)
	now := time.Now().UnixNano()
	st.AppendBatch(&aggregv1.FlowEventBatch{
		NodeName: "node-a",
		AgentId:  "agent-a",
		Events:   []*aggregv1.FlowEvent{sampleEvent(now, 0, 0)},
	}, nil)
	st.AppendBatch(&aggregv1.FlowEventBatch{
		NodeName: "node-b",
		AgentId:  "agent-b",
		Events:   []*aggregv1.FlowEvent{sampleEvent(now+1, 0, 0)},
	}, nil)

	node, agent, events, ok := st.PeekUnsentBatch(100)
	if !ok || len(events) != 1 {
		t.Fatalf("peek = ok:%v len:%d", ok, len(events))
	}
	if node != "node-a" || agent != "agent-a" {
		t.Fatalf("peek node/agent = %q/%q", node, agent)
	}

	st.AdvanceSendCursor(1)
	node, agent, events, ok = st.PeekUnsentBatch(100)
	if !ok || len(events) != 1 {
		t.Fatalf("second peek = ok:%v len:%d", ok, len(events))
	}
	if node != "node-b" || agent != "agent-b" {
		t.Fatalf("second peek node/agent = %q/%q", node, agent)
	}
}

func TestAgePruneDropsUnsentEvents(t *testing.T) {
	st := NewStore(100, time.Millisecond)
	now := time.Now().UnixNano()
	st.AppendBatch(&aggregv1.FlowEventBatch{
		NodeName: "node-a",
		AgentId:  "agent-a",
		Events:   []*aggregv1.FlowEvent{sampleEvent(now-time.Second.Nanoseconds(), 0, 0)},
	}, nil)

	time.Sleep(2 * time.Millisecond)

	st.AppendBatch(&aggregv1.FlowEventBatch{
		NodeName: "node-a",
		AgentId:  "agent-a",
		Events:   []*aggregv1.FlowEvent{sampleEvent(time.Now().UnixNano(), 0, 0)},
	}, nil)

	if st.UnsentCount() != 1 {
		t.Fatalf("unsent = %d, want 1", st.UnsentCount())
	}
	dropped, reason := st.TakeDroppedUnsent()
	if dropped != 1 {
		t.Fatalf("dropped unsent = %d, want 1", dropped)
	}
	if reason != "max_age" {
		t.Fatalf("drop reason = %q", reason)
	}
}

func TestListEventsFilters(t *testing.T) {
	st := NewStore(1000, time.Hour)
	now := time.Now().UnixNano()
	st.AppendBatch(&aggregv1.FlowEventBatch{
		Events: []*aggregv1.FlowEvent{
			{
				EventKind:         aggregv1.FlowEventKind_FLOW_EVENT_KIND_CONNECT,
				TimestampUnixNano: now,
				Src:               &aggregv1.WorkloadRef{Namespace: "insights", Pod: "a"},
				Dst:               &aggregv1.Endpoint{Addr: "10.0.0.1", Port: 80},
			},
			{
				EventKind:         aggregv1.FlowEventKind_FLOW_EVENT_KIND_TRAFFIC,
				TimestampUnixNano: now + int64(time.Second),
				Src:               &aggregv1.WorkloadRef{Namespace: "kube-system", Pod: "b"},
				Dst:               &aggregv1.Endpoint{Addr: "10.0.0.2", Port: 443},
				BytesSent:         10,
			},
		},
	}, func(event *aggregv1.FlowEvent) Enrichment {
		if event.GetEventKind() == aggregv1.FlowEventKind_FLOW_EVENT_KIND_CONNECT {
			return Enrichment{SrcNamespace: "insights", SrcWorkloadKind: "Job", SrcWorkloadName: "demo-traffic", DstNamespace: "insights", DstKind: "Service", DstName: "demo-server"}
		}
		return Enrichment{SrcNamespace: "kube-system", SrcWorkloadKind: "Pod", SrcWorkloadName: "b"}
	})

	connectEvents := st.ListEvents(ListOpts{EventKind: networkflowv1.FlowEventKind_FLOW_EVENT_KIND_CONNECT})
	if len(connectEvents) != 1 {
		t.Fatalf("connect events = %d", len(connectEvents))
	}
	if connectEvents[0].GetSrcWorkload().GetKind() != "Job" {
		t.Fatalf("src workload kind = %q", connectEvents[0].GetSrcWorkload().GetKind())
	}

	sinceEvents := st.ListEvents(ListOpts{Since: now + int64(time.Millisecond)})
	if len(sinceEvents) != 1 || sinceEvents[0].GetEventKind() != networkflowv1.FlowEventKind_FLOW_EVENT_KIND_TRAFFIC {
		t.Fatalf("since filter = %+v", sinceEvents)
	}
}

func sampleEvent(ts int64, sent, received uint64) *aggregv1.FlowEvent {
	return &aggregv1.FlowEvent{
		EventKind:         aggregv1.FlowEventKind_FLOW_EVENT_KIND_TRAFFIC,
		Protocol:          aggregv1.Protocol_PROTOCOL_TCP,
		TimestampUnixNano: ts,
		Src:               &aggregv1.WorkloadRef{Namespace: "default", Pod: "a"},
		Dst:               &aggregv1.Endpoint{Addr: "10.0.0.1", Port: 80},
		BytesSent:         sent,
		BytesReceived:     received,
	}
}

package upstream

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"

	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/store"
	aggregv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/aggregator/v1"
	networkflowv1 "github.com/fairwindsops/fairwinds-insights/pkg/networkflow/v1"
)

type fakeStream struct {
	sendCalls int
	send      func(*networkflowv1.EnrichedFlowEventBatch) error
}

func (f *fakeStream) Send(msg *networkflowv1.EnrichedFlowEventBatch) error {
	f.sendCalls++
	if f.send != nil {
		return f.send(msg)
	}
	return nil
}

func (f *fakeStream) SendMsg(m any) error {
	msg, ok := m.(*networkflowv1.EnrichedFlowEventBatch)
	if !ok {
		return errors.New("unexpected message type")
	}
	return f.Send(msg)
}

func (f *fakeStream) RecvMsg(m any) error {
	return errors.New("not implemented")
}

func (f *fakeStream) CloseAndRecv() (*networkflowv1.PushAck, error) {
	return &networkflowv1.PushAck{}, nil
}

func (f *fakeStream) Header() (metadata.MD, error) { return nil, nil }

func (f *fakeStream) Trailer() metadata.MD { return nil }

func (f *fakeStream) CloseSend() error { return nil }

func (f *fakeStream) Context() context.Context { return context.Background() }

func TestSendPendingAdvancesCursorOnSuccess(t *testing.T) {
	st := store.NewStore(100, 0)
	appendEvents(t, st, "node-a", "agent-a", 3)

	client := NewClient(Config{BatchSize: 2, Organization: "org", Cluster: "cluster"}, st, nil)
	stream := &fakeStream{}

	if err := client.sendPending(stream); err != nil {
		t.Fatalf("sendPending: %v", err)
	}
	if stream.sendCalls != 2 {
		t.Fatalf("send calls = %d, want 2", stream.sendCalls)
	}
	if st.UnsentCount() != 0 {
		t.Fatalf("unsent = %d, want 0", st.UnsentCount())
	}
	if st.SendCursor() != 3 {
		t.Fatalf("send cursor = %d, want 3", st.SendCursor())
	}
}

func TestSendPendingLeavesCursorOnFailure(t *testing.T) {
	st := store.NewStore(100, 0)
	appendEvents(t, st, "node-a", "agent-a", 3)

	sendErr := errors.New("send failed")
	client := NewClient(Config{BatchSize: 2, Organization: "org", Cluster: "cluster"}, st, nil)
	stream := &fakeStream{
		send: func(msg *networkflowv1.EnrichedFlowEventBatch) error {
			if len(msg.GetEvents()) == 1 {
				return sendErr
			}
			return nil
		},
	}

	err := client.sendPending(stream)
	if !errors.Is(err, sendErr) {
		t.Fatalf("sendPending err = %v, want %v", err, sendErr)
	}
	if st.UnsentCount() != 1 {
		t.Fatalf("unsent = %d, want 1", st.UnsentCount())
	}
	if st.SendCursor() != 2 {
		t.Fatalf("send cursor = %d, want 2", st.SendCursor())
	}
}

func TestSendPendingRetriesUnsentAfterFailure(t *testing.T) {
	st := store.NewStore(100, 0)
	appendEvents(t, st, "node-a", "agent-a", 3)

	failOnce := true
	client := NewClient(Config{BatchSize: 2, Organization: "org", Cluster: "cluster"}, st, nil)
	stream := &fakeStream{
		send: func(msg *networkflowv1.EnrichedFlowEventBatch) error {
			if failOnce && len(msg.GetEvents()) == 1 {
				failOnce = false
				return errors.New("send failed")
			}
			return nil
		},
	}

	if err := client.sendPending(stream); err == nil {
		t.Fatal("first sendPending should fail")
	}
	if st.UnsentCount() != 1 {
		t.Fatalf("unsent after failure = %d, want 1", st.UnsentCount())
	}

	stream.sendCalls = 0
	if err := client.sendPending(stream); err != nil {
		t.Fatalf("second sendPending: %v", err)
	}
	if st.UnsentCount() != 0 {
		t.Fatalf("unsent after retry = %d, want 0", st.UnsentCount())
	}
}

func appendEvents(t *testing.T, st *store.Store, nodeName, agentID string, count int) {
	t.Helper()
	now := time.Now().UnixNano()
	events := make([]*aggregv1.FlowEvent, 0, count)
	for i := 0; i < count; i++ {
		events = append(events, &aggregv1.FlowEvent{
			EventKind:         aggregv1.FlowEventKind_FLOW_EVENT_KIND_TRAFFIC,
			Protocol:          aggregv1.Protocol_PROTOCOL_TCP,
			TimestampUnixNano: now + int64(i),
			Src:               &aggregv1.WorkloadRef{Namespace: "default", Pod: "pod-a"},
			Dst:               &aggregv1.Endpoint{Addr: "10.0.0.1", Port: 80},
		})
	}
	if got, _ := st.AppendBatch(&aggregv1.FlowEventBatch{
		NodeName: nodeName,
		AgentId:  agentID,
		Events:   events,
	}, nil); got != int64(count) {
		t.Fatalf("accepted = %d, want %d", got, count)
	}
}

package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"

	aggregv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/aggregator/v1"
)

func TestClientUsesLongLivedStream(t *testing.T) {
	var batches atomic.Int32
	var events atomic.Int32

	_, _, grpcServer, addr := startTestCollector(t, func(batch *aggregv1.FlowEventBatch) {
		batches.Add(1)
		events.Add(int32(len(batch.GetEvents())))
	})
	defer grpcServer.Stop()

	client := newTestClient(addr)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.Run(ctx)

	client.Enqueue(sampleEvent("pod-1"))
	client.Enqueue(sampleEvent("pod-2"))
	waitFor(t, 2*time.Second, func() bool { return events.Load() >= 2 })

	client.Enqueue(sampleEvent("pod-3"))
	client.Flush()
	waitFor(t, 2*time.Second, func() bool { return events.Load() >= 3 })

	if batches.Load() < 2 {
		t.Fatalf("expected multiple batches on one stream, got %d", batches.Load())
	}
}

func TestClientReconnectsWhenCollectorBecomesAvailable(t *testing.T) {
	var events atomic.Int32

	srv := &batchCountingServer{
		onBatch: func(batch *aggregv1.FlowEventBatch) {
			events.Add(int32(len(batch.GetEvents())))
		},
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := lis.Addr().String()

	client := newTestClient(addr)
	client.cfg.MaxPendingEvents = 10
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.Run(ctx)

	client.Enqueue(sampleEvent("pod-1"))
	client.Enqueue(sampleEvent("pod-2"))
	time.Sleep(150 * time.Millisecond)
	if events.Load() != 0 {
		t.Fatalf("expected no events before collector starts, got %d", events.Load())
	}
	if client.PendingCount() > 10 {
		t.Fatalf("pending exceeded cap during outage: %d", client.PendingCount())
	}

	grpcServer := grpc.NewServer()
	aggregv1.RegisterAgentIngestServer(grpcServer, srv)
	go func() {
		_ = grpcServer.Serve(lis)
	}()

	waitFor(t, 5*time.Second, func() bool { return events.Load() >= 2 })
	grpcServer.Stop()
}

func TestRequeuePreservesOrder(t *testing.T) {
	c := NewClient(ClientConfig{NodeName: "n", AgentID: "a", MaxPendingEvents: 100}, nil)
	c.requeue(&aggregv1.FlowEventBatch{Events: []*aggregv1.FlowEvent{{Src: &aggregv1.WorkloadRef{Pod: "b"}}}})
	c.requeue(&aggregv1.FlowEventBatch{Events: []*aggregv1.FlowEvent{{Src: &aggregv1.WorkloadRef{Pod: "a"}}}})

	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.events) != 2 || c.events[0].GetSrc().GetPod() != "a" || c.events[1].GetSrc().GetPod() != "b" {
		t.Fatalf("unexpected queue order: %+v", c.events)
	}
}

func TestEnqueueEnforcesMaxPending(t *testing.T) {
	c := NewClient(ClientConfig{NodeName: "n", AgentID: "a", MaxPendingEvents: 3}, nil)
	for i := 0; i < 5; i++ {
		c.Enqueue(sampleEvent(fmt.Sprintf("pod-%d", i)))
	}

	if c.PendingCount() != 3 {
		t.Fatalf("pending = %d, want 3", c.PendingCount())
	}
	if c.DroppedPending() != 2 {
		t.Fatalf("dropped = %d, want 2", c.DroppedPending())
	}

	c.mu.Lock()
	pods := make([]string, len(c.events))
	for i, e := range c.events {
		pods[i] = e.GetSrc().GetPod()
	}
	c.mu.Unlock()

	want := []string{"pod-2", "pod-3", "pod-4"}
	for i, pod := range want {
		if pods[i] != pod {
			t.Fatalf("pods[%d] = %q, want %q (all: %v)", i, pods[i], pod, pods)
		}
	}
}

func TestRequeueRespectsMaxPending(t *testing.T) {
	c := NewClient(ClientConfig{NodeName: "n", AgentID: "a", MaxPendingEvents: 3}, nil)
	c.Enqueue(sampleEvent("pod-1"))
	c.Enqueue(sampleEvent("pod-2"))
	c.Enqueue(sampleEvent("pod-3"))

	c.requeue(&aggregv1.FlowEventBatch{
		Events: []*aggregv1.FlowEvent{
			{Src: &aggregv1.WorkloadRef{Pod: "requeue-a"}},
			{Src: &aggregv1.WorkloadRef{Pod: "requeue-b"}},
		},
	})

	if c.PendingCount() != 3 {
		t.Fatalf("pending = %d, want 3", c.PendingCount())
	}
	if c.DroppedPending() != 2 {
		t.Fatalf("dropped = %d, want 2", c.DroppedPending())
	}

	c.mu.Lock()
	pods := make([]string, len(c.events))
	for i, e := range c.events {
		pods[i] = e.GetSrc().GetPod()
	}
	c.mu.Unlock()
	want := []string{"pod-1", "pod-2", "pod-3"}
	for i, pod := range want {
		if pods[i] != pod {
			t.Fatalf("pods[%d] = %q, want %q (all: %v)", i, pods[i], pod, pods)
		}
	}
}

func TestSendPendingChunks(t *testing.T) {
	var batchSizes []int
	var sizesMu sync.Mutex

	_, _, grpcServer, addr := startTestCollector(t, func(batch *aggregv1.FlowEventBatch) {
		sizesMu.Lock()
		batchSizes = append(batchSizes, len(batch.GetEvents()))
		sizesMu.Unlock()
	})
	defer grpcServer.Stop()

	client := NewClient(ClientConfig{
		CollectorAddr:       addr,
		NodeName:            "node-a",
		AgentID:             "agent-a",
		BatchSize:           2,
		MaxPendingEvents:    100,
		FlushInterval:       time.Hour,
		ReconnectBackoffMin: 50 * time.Millisecond,
		ReconnectBackoffMax: 200 * time.Millisecond,
	}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go client.Run(ctx)

	for i := 0; i < 5; i++ {
		client.Enqueue(sampleEvent(fmt.Sprintf("pod-%d", i)))
	}
	client.Flush()
	waitFor(t, 3*time.Second, func() bool {
		sizesMu.Lock()
		defer sizesMu.Unlock()
		total := 0
		for _, n := range batchSizes {
			total += n
		}
		return total >= 5
	})

	sizesMu.Lock()
	got := append([]int(nil), batchSizes...)
	sizesMu.Unlock()

	want := []int{2, 2, 1}
	if len(got) != len(want) {
		t.Fatalf("batch count = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("batch[%d] size = %d, want %d (all: %v)", i, got[i], want[i], got)
		}
	}
}

func TestDroppedPendingLogged(t *testing.T) {
	var logBuf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	c := NewClient(ClientConfig{NodeName: "n", AgentID: "a", MaxPendingEvents: 2, BatchSize: 100}, log)
	c.Enqueue(sampleEvent("pod-1"))
	c.Enqueue(sampleEvent("pod-2"))
	c.Enqueue(sampleEvent("pod-3"))

	_, _, grpcServer, addr := startTestCollector(t, nil)
	defer grpcServer.Stop()

	c.cfg.CollectorAddr = addr
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	c.Flush()
	waitFor(t, 3*time.Second, func() bool {
		return bytes.Contains(logBuf.Bytes(), []byte("pending flow events dropped by retention"))
	})

	if !bytes.Contains(logBuf.Bytes(), []byte("reason=max_pending_events")) {
		t.Fatalf("expected drop reason in logs: %s", logBuf.String())
	}
}

func newTestClient(addr string) *Client {
	return NewClient(ClientConfig{
		CollectorAddr:       addr,
		NodeName:            "node-a",
		AgentID:             "agent-a",
		BatchSize:           2,
		MaxPendingEvents:    10,
		FlushInterval:       50 * time.Millisecond,
		ReconnectBackoffMin: 50 * time.Millisecond,
		ReconnectBackoffMax: 200 * time.Millisecond,
	}, nil)
}

func startTestCollector(t *testing.T, onBatch func(*aggregv1.FlowEventBatch)) (*batchCountingServer, net.Listener, *grpc.Server, string) {
	t.Helper()

	srv := &batchCountingServer{onBatch: onBatch}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	aggregv1.RegisterAgentIngestServer(grpcServer, srv)
	go func() {
		_ = grpcServer.Serve(lis)
	}()

	return srv, lis, grpcServer, lis.Addr().String()
}

func sampleEvent(pod string) *aggregv1.FlowEvent {
	return &aggregv1.FlowEvent{
		EventKind:         aggregv1.FlowEventKind_FLOW_EVENT_KIND_CONNECT,
		Protocol:          aggregv1.Protocol_PROTOCOL_TCP,
		TimestampUnixNano: time.Now().UnixNano(),
		Src:               &aggregv1.WorkloadRef{Namespace: "default", Pod: pod},
		Dst:               &aggregv1.Endpoint{Addr: "10.0.0.1", Port: 443},
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

type batchCountingServer struct {
	aggregv1.UnimplementedAgentIngestServer
	onBatch func(*aggregv1.FlowEventBatch)
}

func (s *batchCountingServer) PushEvents(stream aggregv1.AgentIngest_PushEventsServer) error {
	var total int64
	for {
		batch, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&aggregv1.PushAck{AcceptedEvents: total})
		}
		if err != nil {
			return err
		}
		if s.onBatch != nil {
			s.onBatch(batch)
		}
		total += int64(len(batch.GetEvents()))
	}
}

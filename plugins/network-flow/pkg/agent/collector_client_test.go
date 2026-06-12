package agent

import (
	"context"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"

	flowv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow/pkg/flow/v1"
)

func TestClientUsesLongLivedStream(t *testing.T) {
	var batches atomic.Int32
	var events atomic.Int32

	_, _, grpcServer, addr := startTestCollector(t, func(batch *flowv1.FlowEventBatch) {
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
		onBatch: func(batch *flowv1.FlowEventBatch) {
			events.Add(int32(len(batch.GetEvents())))
		},
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := lis.Addr().String()

	client := newTestClient(addr)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.Run(ctx)

	client.Enqueue(sampleEvent("pod-1"))
	client.Enqueue(sampleEvent("pod-2"))
	time.Sleep(150 * time.Millisecond)
	if events.Load() != 0 {
		t.Fatalf("expected no events before collector starts, got %d", events.Load())
	}

	grpcServer := grpc.NewServer()
	flowv1.RegisterFlowIngestServer(grpcServer, srv)
	go func() {
		_ = grpcServer.Serve(lis)
	}()

	waitFor(t, 5*time.Second, func() bool { return events.Load() >= 2 })
	grpcServer.Stop()
}

func TestRequeuePreservesOrder(t *testing.T) {
	c := NewClient(ClientConfig{NodeName: "n", AgentID: "a"}, nil)
	c.requeue(&flowv1.FlowEventBatch{Events: []*flowv1.FlowEvent{{Src: &flowv1.WorkloadRef{Pod: "b"}}}})
	c.requeue(&flowv1.FlowEventBatch{Events: []*flowv1.FlowEvent{{Src: &flowv1.WorkloadRef{Pod: "a"}}}})

	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.events) != 2 || c.events[0].GetSrc().GetPod() != "a" || c.events[1].GetSrc().GetPod() != "b" {
		t.Fatalf("unexpected queue order: %+v", c.events)
	}
}

func newTestClient(addr string) *Client {
	return NewClient(ClientConfig{
		CollectorAddr:       addr,
		NodeName:            "node-a",
		AgentID:             "agent-a",
		BatchSize:           2,
		FlushInterval:       50 * time.Millisecond,
		ReconnectBackoffMin: 50 * time.Millisecond,
		ReconnectBackoffMax: 200 * time.Millisecond,
	}, nil)
}

func startTestCollector(t *testing.T, onBatch func(*flowv1.FlowEventBatch)) (*batchCountingServer, net.Listener, *grpc.Server, string) {
	t.Helper()

	srv := &batchCountingServer{onBatch: onBatch}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	flowv1.RegisterFlowIngestServer(grpcServer, srv)
	go func() {
		_ = grpcServer.Serve(lis)
	}()

	return srv, lis, grpcServer, lis.Addr().String()
}

func sampleEvent(pod string) *flowv1.FlowEvent {
	return &flowv1.FlowEvent{
		EventKind:         flowv1.FlowEventKind_FLOW_EVENT_KIND_CONNECT,
		Protocol:          flowv1.Protocol_PROTOCOL_TCP,
		TimestampUnixNano: time.Now().UnixNano(),
		Src:               &flowv1.WorkloadRef{Namespace: "default", Pod: pod},
		Dst:               &flowv1.Endpoint{Addr: "10.0.0.1", Port: 443},
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
	flowv1.UnimplementedFlowIngestServer
	onBatch func(*flowv1.FlowEventBatch)
}

func (s *batchCountingServer) PushEvents(stream flowv1.FlowIngest_PushEventsServer) error {
	var total int64
	for {
		batch, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&flowv1.PushAck{AcceptedEvents: total})
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

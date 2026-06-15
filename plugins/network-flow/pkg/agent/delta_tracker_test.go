package agent

import "testing"

func TestByteDeltaTrackerFirstObservationIsBaseline(t *testing.T) {
	tracker := NewByteDeltaTracker()
	fields := TCPFields{Pod: "a", DstAddr: "10.0.0.1", DstPort: 80, BytesSent: 100, BytesReceived: 200}

	tracker.BeginPoll()
	delta, ok := tracker.Observe(fields)
	if ok || delta.Sent != 0 || delta.Received != 0 {
		t.Fatalf("baseline = (%+v, %v)", delta, ok)
	}
	if flushed := tracker.EndPoll(); len(flushed) != 0 {
		t.Fatalf("expected no flush while connection is still active, got %+v", flushed)
	}
}

func TestByteDeltaTrackerEmitsDelta(t *testing.T) {
	tracker := NewByteDeltaTracker()

	tracker.BeginPoll()
	tracker.Observe(TCPFields{Pod: "a", DstAddr: "10.0.0.1", DstPort: 80, BytesSent: 100, BytesReceived: 200})
	tracker.EndPoll()

	tracker.BeginPoll()
	delta, ok := tracker.Observe(TCPFields{Pod: "a", DstAddr: "10.0.0.1", DstPort: 80, BytesSent: 150, BytesReceived: 260})
	if !ok {
		t.Fatal("expected delta")
	}
	if delta.Sent != 50 || delta.Received != 60 {
		t.Fatalf("delta = (%d, %d)", delta.Sent, delta.Received)
	}
}

func TestByteDeltaTrackerFlushesShortLivedConnection(t *testing.T) {
	tracker := NewByteDeltaTracker()
	fields := TCPFields{Pod: "a", DstAddr: "10.0.0.1", DstPort: 80, BytesSent: 500, BytesReceived: 120}

	tracker.BeginPoll()
	if _, ok := tracker.Observe(fields); ok {
		t.Fatal("expected baseline on first poll")
	}
	if flushed := tracker.EndPoll(); len(flushed) != 0 {
		t.Fatal("expected no flush on first poll")
	}

	tracker.BeginPoll()
	flushed := tracker.EndPoll()
	if len(flushed) != 1 {
		t.Fatalf("expected one flushed connection, got %d", len(flushed))
	}
	if flushed[0].Sent != 500 || flushed[0].Received != 120 {
		t.Fatalf("flush = (%d, %d)", flushed[0].Sent, flushed[0].Received)
	}
}

func TestByteDeltaTrackerHandlesReset(t *testing.T) {
	tracker := NewByteDeltaTracker()
	key := connKey{Pod: "a", DstAddr: "10.0.0.1", DstPort: 80}

	tracker.Delta(key, 1000, 2000)
	sent, recv, ok := tracker.Delta(key, 50, 80)
	if !ok {
		t.Fatal("expected delta")
	}
	if sent != 50 || recv != 80 {
		t.Fatalf("reset delta = (%d, %d)", sent, recv)
	}
}

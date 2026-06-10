package agent

import "testing"

func TestByteDeltaTrackerFirstObservationIsBaseline(t *testing.T) {
	tracker := NewByteDeltaTracker()
	key := connKey{Pod: "a", DstAddr: "10.0.0.1", DstPort: 80}

	sent, recv, ok := tracker.Delta(key, 100, 200)
	if ok || sent != 0 || recv != 0 {
		t.Fatalf("baseline = (%d, %d, %v)", sent, recv, ok)
	}
}

func TestByteDeltaTrackerEmitsDelta(t *testing.T) {
	tracker := NewByteDeltaTracker()
	key := connKey{Pod: "a", DstAddr: "10.0.0.1", DstPort: 80}

	tracker.Delta(key, 100, 200)
	sent, recv, ok := tracker.Delta(key, 150, 260)
	if !ok {
		t.Fatal("expected delta")
	}
	if sent != 50 || recv != 60 {
		t.Fatalf("delta = (%d, %d)", sent, recv)
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

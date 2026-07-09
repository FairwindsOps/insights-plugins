package peerindex

import (
	"testing"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/kube"
)

func TestIndexPutLookup(t *testing.T) {
	idx := New(time.Minute)
	now := time.Now()
	backend := kube.BackendIdentity{
		PodNamespace:       "payments",
		PodName:            "backend-abc",
		WorkloadNamespace:  "payments",
		WorkloadKind:       "Deployment",
		WorkloadName:       "backend",
		ServiceNamespace:   "payments",
		ServiceName:        "backend",
	}
	idx.Put("10.244.0.94", 55444, backend, now)

	got, ok := idx.Lookup("10.244.0.94", 55444)
	if !ok {
		t.Fatal("expected lookup hit")
	}
	if got.PodName != "backend-abc" || got.ServiceName != "backend" {
		t.Fatalf("backend = %#v", got)
	}
}

func TestIndexExpiresEntries(t *testing.T) {
	idx := New(time.Millisecond)
	now := time.Now()
	idx.Put("10.244.0.94", 55444, kube.BackendIdentity{PodName: "backend-abc"}, now)
	time.Sleep(2 * time.Millisecond)
	if _, ok := idx.Lookup("10.244.0.94", 55444); ok {
		t.Fatal("expected expired entry")
	}
}

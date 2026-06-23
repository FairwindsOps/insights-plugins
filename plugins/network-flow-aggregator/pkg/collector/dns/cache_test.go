package dns

import (
	"testing"
	"time"
)

func TestCacheRecordAndLookupWorkloadScoped(t *testing.T) {
	c := NewCache(time.Minute)
	ts := time.Now()
	c.RecordResponse("default", "frontend", "api.stripe.com", "A", "Success", []string{"104.21.11.16"}, ts)

	host, ok := c.Lookup("default", "frontend", "104.21.11.16")
	if !ok || host != "api.stripe.com" {
		t.Fatalf("lookup = %q ok=%v", host, ok)
	}
}

func TestCacheClusterFallback(t *testing.T) {
	c := NewCache(time.Minute)
	ts := time.Now()
	c.RecordResponse("default", "frontend", "api.stripe.com", "A", "Success", []string{"104.21.11.16"}, ts)

	host, ok := c.Lookup("shop", "other-pod", "104.21.11.16")
	if !ok || host != "api.stripe.com" {
		t.Fatalf("cluster fallback = %q ok=%v", host, ok)
	}
}

func TestCacheSkipsInternalNames(t *testing.T) {
	c := NewCache(time.Minute)
	c.RecordResponse("default", "frontend", "postgres.default.svc.cluster.local", "A", "Success", []string{"10.96.0.5"}, time.Now())
	if _, ok := c.Lookup("default", "frontend", "10.96.0.5"); ok {
		t.Fatal("expected internal hostname to be skipped")
	}
}

func TestCacheSkipsCoreDNSPods(t *testing.T) {
	c := NewCache(time.Minute)
	c.RecordResponse("kube-system", "coredns-abc", "example.com", "A", "Success", []string{"93.184.216.34"}, time.Now())
	if _, ok := c.Lookup("default", "frontend", "93.184.216.34"); ok {
		t.Fatal("expected coredns pod responses to be skipped")
	}
}

func TestCachePrunesExpired(t *testing.T) {
	c := NewCache(time.Millisecond)
	ts := time.Now().Add(-2 * time.Millisecond)
	c.RecordResponse("default", "frontend", "example.com", "A", "Success", []string{"93.184.216.34"}, ts)
	c.pruneLocked(time.Now())
	if _, ok := c.Lookup("default", "frontend", "93.184.216.34"); ok {
		t.Fatal("expected expired entry to be pruned")
	}
}

func TestCacheMultipleAddresses(t *testing.T) {
	c := NewCache(time.Minute)
	c.RecordResponse("default", "frontend", "example.com", "A", "Success", []string{"1.1.1.1", "1.0.0.1"}, time.Now())
	for _, ip := range []string{"1.1.1.1", "1.0.0.1"} {
		if host, ok := c.Lookup("default", "frontend", ip); !ok || host != "example.com" {
			t.Fatalf("lookup %s = %q ok=%v", ip, host, ok)
		}
	}
}

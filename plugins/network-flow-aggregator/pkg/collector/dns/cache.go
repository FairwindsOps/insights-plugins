package dns

import (
	"strings"
	"sync"
	"time"
)

type workloadIPKey struct {
	namespace string
	pod       string
	ip        string
}

type cacheEntry struct {
	hostname string
	seenAt   time.Time
}

type Cache struct {
	mu         sync.RWMutex
	byWorkload map[workloadIPKey]cacheEntry
	byIP       map[string]cacheEntry
	maxAge     time.Duration
}

func NewCache(maxAge time.Duration) *Cache {
	if maxAge <= 0 {
		maxAge = 15 * time.Minute
	}
	return &Cache{
		byWorkload: make(map[workloadIPKey]cacheEntry),
		byIP:       make(map[string]cacheEntry),
		maxAge:     maxAge,
	}
}

func (c *Cache) RecordResponse(srcNs, srcPod, hostname, qtype, rcode string, addresses []string, ts time.Time) {
	if c == nil || len(addresses) == 0 || !isCacheableResponse(qtype, rcode) {
		return
	}
	hostname = strings.TrimSuffix(strings.TrimSpace(hostname), ".")
	if hostname == "" || isInternalHostname(hostname) || isCoreDNSPod(srcNs, srcPod) {
		return
	}
	if ts.IsZero() {
		ts = time.Now()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, ip := range addresses {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		entry := cacheEntry{hostname: hostname, seenAt: ts}
		c.byWorkload[workloadIPKey{namespace: srcNs, pod: srcPod, ip: ip}] = entry
		c.byIP[ip] = entry
	}
	c.pruneLocked(ts)
}

func (c *Cache) Lookup(ns, pod, ip string) (string, bool) {
	if c == nil || ip == "" {
		return "", false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	if entry, ok := c.byWorkload[workloadIPKey{namespace: ns, pod: pod, ip: ip}]; ok && !c.expired(entry, now) {
		return entry.hostname, true
	}
	if entry, ok := c.byIP[ip]; ok && !c.expired(entry, now) {
		return entry.hostname, true
	}
	return "", false
}

func (c *Cache) pruneLocked(now time.Time) {
	for k, entry := range c.byWorkload {
		if c.expired(entry, now) {
			delete(c.byWorkload, k)
		}
	}
	for ip, entry := range c.byIP {
		if c.expired(entry, now) {
			delete(c.byIP, ip)
		}
	}
}

func (c *Cache) expired(entry cacheEntry, now time.Time) bool {
	return c.maxAge > 0 && now.Sub(entry.seenAt) > c.maxAge
}

func isInternalHostname(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".svc.cluster.local") || strings.HasSuffix(lower, ".cluster.local")
}

func isCoreDNSPod(ns, pod string) bool {
	return ns == "kube-system" && strings.HasPrefix(pod, "coredns-")
}

func isCacheableResponse(qtype, rcode string) bool {
	switch strings.ToUpper(strings.TrimSpace(qtype)) {
	case "A", "AAAA", "":
	default:
		return false
	}
	return strings.EqualFold(strings.TrimSpace(rcode), "Success")
}

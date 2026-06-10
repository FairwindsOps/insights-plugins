package store

import (
	"fmt"
	"sync"
	"time"

	flowv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow/pkg/flow/v1"
)

type EdgeKey struct {
	SrcNamespace        string
	SrcWorkloadKind     string
	SrcWorkloadName     string
	DstNamespace        string
	DstKind             string
	DstName             string
	DstPort             uint32
	FlowType            flowv1.FlowType
	BucketStartUnixNano int64
}

type Edge struct {
	Key               EdgeKey `json:"key"`
	Count             int64   `json:"count"`
	BytesSent         int64   `json:"bytes_sent"`
	BytesReceived     int64   `json:"bytes_received"`
	LastSeenUnixNano  int64   `json:"last_seen_unix_nano"`
	FirstSeenUnixNano int64   `json:"first_seen_unix_nano"`
	SrcPod            string  `json:"src_pod,omitempty"`
	SrcContainer      string  `json:"src_container,omitempty"`
	DstAddr           string  `json:"dst_addr,omitempty"`
}

type Enrichment struct {
	SrcNamespace    string
	SrcWorkloadKind string
	SrcWorkloadName string
	DstNamespace    string
	DstKind         string
	DstName         string
	DstAddr         string
}

type Store struct {
	mu             sync.RWMutex
	edges          map[EdgeKey]*Edge
	bucketInterval time.Duration
}

func NewStore(bucketInterval time.Duration) *Store {
	if bucketInterval <= 0 {
		bucketInterval = time.Minute
	}
	return &Store{
		edges:          make(map[EdgeKey]*Edge),
		bucketInterval: bucketInterval,
	}
}

func (s *Store) BucketInterval() time.Duration {
	return s.bucketInterval
}

func bucketStart(ts int64, interval time.Duration) int64 {
	if ts <= 0 {
		ts = time.Now().UnixNano()
	}
	n := int64(interval)
	return ts - (ts % n)
}

func edgeKeyFromFlow(f *flowv1.NetworkFlow, enrich Enrichment, bucketInterval time.Duration) EdgeKey {
	dstName := enrich.DstName
	if enrich.DstKind == "" {
		dstName = enrich.DstAddr
	}
	ts := f.GetTimestampUnixNano()
	return EdgeKey{
		SrcNamespace:        enrich.SrcNamespace,
		SrcWorkloadKind:     enrich.SrcWorkloadKind,
		SrcWorkloadName:     enrich.SrcWorkloadName,
		DstNamespace:        enrich.DstNamespace,
		DstKind:             enrich.DstKind,
		DstName:             dstName,
		DstPort:             f.GetDst().GetPort(),
		FlowType:            f.GetType(),
		BucketStartUnixNano: bucketStart(ts, bucketInterval),
	}
}

func (s *Store) IngestBatch(batch *flowv1.FlowBatch, enrich func(*flowv1.NetworkFlow) Enrichment) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	var accepted int64
	for _, f := range batch.GetFlows() {
		if f == nil || f.GetSrc().GetPod() == "" || f.GetDst().GetAddr() == "" {
			continue
		}

		var enrichment Enrichment
		if enrich != nil {
			enrichment = enrich(f)
		} else {
			enrichment = Enrichment{
				SrcNamespace:    f.GetSrc().GetNamespace(),
				SrcWorkloadKind: "Pod",
				SrcWorkloadName: f.GetSrc().GetPod(),
				DstAddr:         f.GetDst().GetAddr(),
			}
		}

		key := edgeKeyFromFlow(f, enrichment, s.bucketInterval)
		ts := f.GetTimestampUnixNano()
		if ts == 0 {
			ts = time.Now().UnixNano()
		}

		bytesSent := int64(f.GetBytesSent())
		bytesReceived := int64(f.GetBytesReceived())

		edge, ok := s.edges[key]
		if !ok {
			s.edges[key] = &Edge{
				Key:               key,
				Count:             connectionCount(bytesSent, bytesReceived),
				BytesSent:         bytesSent,
				BytesReceived:     bytesReceived,
				FirstSeenUnixNano: ts,
				LastSeenUnixNano:  ts,
				SrcPod:            f.GetSrc().GetPod(),
				SrcContainer:      f.GetSrc().GetContainer(),
				DstAddr:           f.GetDst().GetAddr(),
			}
		} else {
			edge.BytesSent += bytesSent
			edge.BytesReceived += bytesReceived
			edge.Count += connectionCount(bytesSent, bytesReceived)
			edge.LastSeenUnixNano = ts
			if edge.FirstSeenUnixNano == 0 || ts < edge.FirstSeenUnixNano {
				edge.FirstSeenUnixNano = ts
			}
			if edge.SrcContainer == "" {
				edge.SrcContainer = f.GetSrc().GetContainer()
			}
		}
		accepted++
	}
	return accepted
}

func connectionCount(bytesSent, bytesReceived int64) int64 {
	if bytesSent == 0 && bytesReceived == 0 {
		return 1
	}
	return 0
}

func (s *Store) ListEdges() []Edge {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Edge, 0, len(s.edges))
	for _, e := range s.edges {
		out = append(out, *e)
	}
	return out
}

func (k EdgeKey) String() string {
	src := fmt.Sprintf("%s/%s/%s", k.SrcNamespace, k.SrcWorkloadKind, k.SrcWorkloadName)
	dst := k.DstName
	if k.DstKind != "" {
		dst = fmt.Sprintf("%s/%s/%s", k.DstNamespace, k.DstKind, k.DstName)
	}
	return fmt.Sprintf("%s->%s:%d:%s@%d", src, dst, k.DstPort, k.FlowType.String(), k.BucketStartUnixNano)
}

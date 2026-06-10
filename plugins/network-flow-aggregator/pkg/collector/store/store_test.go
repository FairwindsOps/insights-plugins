package store

import (
	"testing"
	"time"

	flowv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow/pkg/flow/v1"
)

func TestIngestBatchAggregatesEdges(t *testing.T) {
	st := NewStore(time.Minute)
	batch := &flowv1.FlowBatch{
		Flows: []*flowv1.NetworkFlow{
			{
				Type:              flowv1.FlowType_FLOW_TYPE_TCP,
				TimestampUnixNano: 100,
				Src:               &flowv1.WorkloadEndpoint{Namespace: "default", Pod: "a"},
				Dst:               &flowv1.NetworkEndpoint{Addr: "10.0.0.1", Port: 80},
			},
			{
				Type:              flowv1.FlowType_FLOW_TYPE_TCP,
				TimestampUnixNano: 200,
				Src:               &flowv1.WorkloadEndpoint{Namespace: "default", Pod: "a"},
				Dst:               &flowv1.NetworkEndpoint{Addr: "10.0.0.1", Port: 80},
			},
		},
	}

	enrich := func(f *flowv1.NetworkFlow) Enrichment {
		return Enrichment{
			SrcNamespace:    f.GetSrc().GetNamespace(),
			SrcWorkloadKind: "Deployment",
			SrcWorkloadName: "payments",
			DstAddr:         f.GetDst().GetAddr(),
		}
	}

	if got := st.IngestBatch(batch, enrich); got != 2 {
		t.Fatalf("accepted = %d", got)
	}
	edges := st.ListEdges()
	if len(edges) != 1 {
		t.Fatalf("edges = %d", len(edges))
	}
	if edges[0].Count != 2 {
		t.Fatalf("count = %d", edges[0].Count)
	}
	if edges[0].Key.SrcWorkloadKind != "Deployment" || edges[0].Key.SrcWorkloadName != "payments" {
		t.Fatalf("workload key = %#v", edges[0].Key)
	}
	if edges[0].FirstSeenUnixNano != 100 || edges[0].LastSeenUnixNano != 200 {
		t.Fatalf("seen range = %d..%d", edges[0].FirstSeenUnixNano, edges[0].LastSeenUnixNano)
	}
}

func TestIngestBatchSumsBytes(t *testing.T) {
	st := NewStore(time.Minute)
	batch := &flowv1.FlowBatch{
		Flows: []*flowv1.NetworkFlow{
			{
				Type:              flowv1.FlowType_FLOW_TYPE_TCP,
				TimestampUnixNano: 100,
				Src:               &flowv1.WorkloadEndpoint{Namespace: "default", Pod: "a"},
				Dst:               &flowv1.NetworkEndpoint{Addr: "10.0.0.1", Port: 80},
				BytesSent:         100,
				BytesReceived:     200,
			},
			{
				Type:              flowv1.FlowType_FLOW_TYPE_TCP,
				TimestampUnixNano: 200,
				Src:               &flowv1.WorkloadEndpoint{Namespace: "default", Pod: "a"},
				Dst:               &flowv1.NetworkEndpoint{Addr: "10.0.0.1", Port: 80},
				BytesSent:         50,
				BytesReceived:     60,
			},
		},
	}

	enrich := func(f *flowv1.NetworkFlow) Enrichment {
		return Enrichment{
			SrcNamespace:    f.GetSrc().GetNamespace(),
			SrcWorkloadKind: "Deployment",
			SrcWorkloadName: "payments",
			DstAddr:         f.GetDst().GetAddr(),
		}
	}

	if got := st.IngestBatch(batch, enrich); got != 2 {
		t.Fatalf("accepted = %d", got)
	}
	edges := st.ListEdges()
	if len(edges) != 1 {
		t.Fatalf("edges = %d", len(edges))
	}
	if edges[0].BytesSent != 150 || edges[0].BytesReceived != 260 {
		t.Fatalf("bytes = sent:%d recv:%d", edges[0].BytesSent, edges[0].BytesReceived)
	}
	if edges[0].Count != 0 {
		t.Fatalf("count = %d, want 0 for byte-only flows", edges[0].Count)
	}
}

func TestIngestBatchBucketsByTimestamp(t *testing.T) {
	st := NewStore(time.Minute)
	bucket := int64(time.Minute)
	batch := &flowv1.FlowBatch{
		Flows: []*flowv1.NetworkFlow{
			{
				Type:              flowv1.FlowType_FLOW_TYPE_TCP,
				TimestampUnixNano: bucket + 10,
				Src:               &flowv1.WorkloadEndpoint{Namespace: "default", Pod: "a"},
				Dst:               &flowv1.NetworkEndpoint{Addr: "10.0.0.1", Port: 80},
			},
			{
				Type:              flowv1.FlowType_FLOW_TYPE_TCP,
				TimestampUnixNano: bucket*2 + 10,
				Src:               &flowv1.WorkloadEndpoint{Namespace: "default", Pod: "a"},
				Dst:               &flowv1.NetworkEndpoint{Addr: "10.0.0.1", Port: 80},
			},
		},
	}

	enrich := func(f *flowv1.NetworkFlow) Enrichment {
		return Enrichment{
			SrcNamespace:    f.GetSrc().GetNamespace(),
			SrcWorkloadKind: "Pod",
			SrcWorkloadName: f.GetSrc().GetPod(),
			DstAddr:         f.GetDst().GetAddr(),
		}
	}

	if got := st.IngestBatch(batch, enrich); got != 2 {
		t.Fatalf("accepted = %d", got)
	}
	if len(st.ListEdges()) != 2 {
		t.Fatalf("expected separate buckets")
	}
}

func TestIngestBatchResolvesDstServiceKey(t *testing.T) {
	st := NewStore(time.Minute)
	batch := &flowv1.FlowBatch{
		Flows: []*flowv1.NetworkFlow{
			{
				Type: flowv1.FlowType_FLOW_TYPE_TCP,
				Src:  &flowv1.WorkloadEndpoint{Namespace: "prod", Pod: "payments-abc"},
				Dst:  &flowv1.NetworkEndpoint{Addr: "10.96.0.10", Port: 5432},
			},
		},
	}

	enrich := func(f *flowv1.NetworkFlow) Enrichment {
		return Enrichment{
			SrcNamespace:    "prod",
			SrcWorkloadKind: "Deployment",
			SrcWorkloadName: "payments",
			DstNamespace:    "prod",
			DstKind:         "Service",
			DstName:         "postgres",
			DstAddr:         f.GetDst().GetAddr(),
		}
	}

	if got := st.IngestBatch(batch, enrich); got != 1 {
		t.Fatalf("accepted = %d", got)
	}
	edges := st.ListEdges()
	if len(edges) != 1 {
		t.Fatalf("edges = %d", len(edges))
	}
	key := edges[0].Key
	if key.DstKind != "Service" || key.DstName != "postgres" || key.DstNamespace != "prod" {
		t.Fatalf("dst key = %#v", key)
	}
}

//go:build linux

package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/inspektor-gadget/inspektor-gadget/pkg/datasource"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/operators"

	flowv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow/pkg/flow/v1"
)

const defaultTopTCPImage = "ghcr.io/inspektor-gadget/gadget/top_tcp:v0.52.0"

type TopTCPRunner struct {
	cfg     GadgetConfig
	client  *Client
	tracker *ByteDeltaTracker
	log     *slog.Logger
}

func NewTopTCPRunner(cfg GadgetConfig, client *Client, log *slog.Logger) *TopTCPRunner {
	if cfg.IGAddress == "" {
		cfg.IGAddress = "tcp://127.0.0.1:8080"
	}
	if cfg.GadgetImage == "" {
		cfg.GadgetImage = defaultTopTCPImage
	}
	if log == nil {
		log = slog.Default()
	}
	return &TopTCPRunner{
		cfg:     cfg,
		client:  client,
		tracker: NewByteDeltaTracker(),
		log:     log,
	}
}

func (r *TopTCPRunner) Run(ctx context.Context) error {
	const opPriority = 50000
	return runGadget(ctx, r.cfg, r.log, "top_tcp", func(gctx operators.GadgetContext) error {
		for _, ds := range gctx.GetDataSources() {
			fields := topTCPFields{ds: ds}
			if err := fields.init(); err != nil {
				return err
			}
			ds.Subscribe(func(_ datasource.DataSource, data datasource.Data) error {
				flow := r.mapSnapshot(fields.extract(data))
				r.client.Enqueue(flow)
				return nil
			}, opPriority)
		}
		return nil
	}, nil)
}

func (r *TopTCPRunner) mapSnapshot(fields TCPFields) *flowv1.NetworkFlow {
	if fields.Pod == "" || fields.DstAddr == "" {
		return nil
	}

	key := connKey{
		Namespace: fields.Namespace,
		Pod:       fields.Pod,
		Container: fields.Container,
		SrcAddr:   fields.SrcAddr,
		SrcPort:   fields.SrcPort,
		DstAddr:   fields.DstAddr,
		DstPort:   fields.DstPort,
		PID:       fields.PID,
	}
	sent, received, ok := r.tracker.Delta(key, fields.BytesSent, fields.BytesReceived)
	if !ok {
		return nil
	}

	return MapTCP(TCPFields{
		Namespace:     fields.Namespace,
		Pod:           fields.Pod,
		Container:     fields.Container,
		DstAddr:       fields.DstAddr,
		DstPort:       fields.DstPort,
		Timestamp:     fields.Timestamp,
		BytesSent:     sent,
		BytesReceived: received,
	})
}

type topTCPFields struct {
	ds                        datasource.DataSource
	namespace, pod, container datasource.FieldAccessor
	srcAddr, srcPort, srcEp   datasource.FieldAccessor
	dstAddr, dstPort, dstEp   datasource.FieldAccessor
	sent, received            datasource.FieldAccessor
	pid                       datasource.FieldAccessor
	timestamp                 datasource.FieldAccessor
}

func (f *topTCPFields) init() error {
	f.namespace = f.ds.GetField("k8s.namespace")
	f.pod = f.ds.GetField("k8s.podName")
	f.container = f.ds.GetField("k8s.containerName")
	f.srcAddr = f.ds.GetField("src.addr")
	f.srcPort = f.ds.GetField("src.port")
	f.srcEp = f.ds.GetField("src.endpoint")
	f.dstAddr = f.ds.GetField("dst.addr")
	f.dstPort = f.ds.GetField("dst.port")
	f.dstEp = f.ds.GetField("dst.endpoint")
	f.sent = f.ds.GetField("sent_raw")
	f.received = f.ds.GetField("received_raw")
	f.pid = f.ds.GetField("pid")
	f.timestamp = f.ds.GetField("timestamp_raw")
	if f.pod == nil {
		return fmt.Errorf("missing k8s.podName field")
	}
	if f.sent == nil || f.received == nil {
		return fmt.Errorf("missing sent_raw/received_raw fields")
	}
	if f.dstAddr == nil && f.dstEp == nil {
		return fmt.Errorf("missing dst endpoint fields")
	}
	return nil
}

func (f *topTCPFields) extract(data datasource.Data) TCPFields {
	out := TCPFields{}
	if f.namespace != nil {
		out.Namespace, _ = f.namespace.String(data)
	}
	if f.pod != nil {
		out.Pod, _ = f.pod.String(data)
	}
	if f.container != nil {
		out.Container, _ = f.container.String(data)
	}
	if f.timestamp != nil {
		ts, _ := f.timestamp.Int64(data)
		out.Timestamp = ts
	}
	if f.srcAddr != nil {
		out.SrcAddr, _ = f.srcAddr.String(data)
	}
	if f.srcPort != nil {
		port, _ := f.srcPort.Uint16(data)
		out.SrcPort = uint32(port)
	}
	if out.SrcAddr == "" && f.srcEp != nil {
		endpoint, _ := f.srcEp.String(data)
		out.SrcAddr, out.SrcPort = splitEndpoint(endpoint)
	}
	if f.dstAddr != nil {
		out.DstAddr, _ = f.dstAddr.String(data)
	}
	if f.dstPort != nil {
		port, _ := f.dstPort.Uint16(data)
		out.DstPort = uint32(port)
	}
	if out.DstAddr == "" && f.dstEp != nil {
		endpoint, _ := f.dstEp.String(data)
		out.DstAddr, out.DstPort = splitEndpoint(endpoint)
	}
	if f.sent != nil {
		out.BytesSent, _ = f.sent.Uint64(data)
	}
	if f.received != nil {
		out.BytesReceived, _ = f.received.Uint64(data)
	}
	if f.pid != nil {
		pid, _ := f.pid.Uint32(data)
		out.PID = pid
	}
	return out
}

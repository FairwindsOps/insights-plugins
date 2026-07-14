//go:build linux

package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/inspektor-gadget/inspektor-gadget/pkg/datasource"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/operators"

	aggregv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/aggregator/v1"
)

type GadgetConfig struct {
	IGAddress   string
	GadgetImage string
}

type TraceTCPRunner struct {
	cfg    GadgetConfig
	client *Client
	log    *slog.Logger
}

func NewTraceTCPRunner(cfg GadgetConfig, client *Client, log *slog.Logger) *TraceTCPRunner {
	if cfg.IGAddress == "" {
		cfg.IGAddress = "tcp://127.0.0.1:8080"
	}
	if log == nil {
		log = slog.Default()
	}
	return &TraceTCPRunner{cfg: cfg, client: client, log: log}
}

func (r *TraceTCPRunner) Run(ctx context.Context) error {
	const opPriority = 50000
	return runGadget(ctx, r.cfg, r.log, "trace_tcp", func(gctx operators.GadgetContext) error {
		for _, ds := range gctx.GetDataSources() {
			fields := traceTCPFields{ds: ds}
			if err := fields.init(); err != nil {
				return err
			}
			err := ds.Subscribe(func(_ datasource.DataSource, data datasource.Data) error {
				raw := fields.extract(data)
				raw.EventKind = aggregv1.FlowEventKind_FLOW_EVENT_KIND_CONNECT
				event := MapFlowEvent(raw)
				r.client.Enqueue(event)
				return nil
			}, opPriority)
			if err != nil {
				return err
			}
		}
		return nil
	}, map[string]string{
		"ebpf.connect-only": "true",
	})
}

type traceTCPFields struct {
	ds                        datasource.DataSource
	namespace, pod, container datasource.FieldAccessor
	srcAddr, srcPort, srcEp   datasource.FieldAccessor
	dstAddr, dstPort, dstEp   datasource.FieldAccessor
	timestamp                 datasource.FieldAccessor
}

func (f *traceTCPFields) init() error {
	f.namespace = f.ds.GetField("k8s.namespace")
	f.pod = f.ds.GetField("k8s.podName")
	f.container = f.ds.GetField("k8s.containerName")
	f.srcAddr = f.ds.GetField("src.addr")
	f.srcPort = f.ds.GetField("src.port")
	f.srcEp = f.ds.GetField("src.endpoint")
	f.dstAddr = f.ds.GetField("dst.addr")
	f.dstPort = f.ds.GetField("dst.port")
	f.dstEp = f.ds.GetField("dst.endpoint")
	f.timestamp = f.ds.GetField("timestamp_raw")
	if f.pod == nil {
		return fmt.Errorf("missing k8s.podName field")
	}
	if f.dstAddr == nil && f.dstEp == nil {
		return fmt.Errorf("missing dst endpoint fields")
	}
	return nil
}

func (f *traceTCPFields) extract(data datasource.Data) TCPFields {
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
	return out
}

//go:build linux

package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/inspektor-gadget/inspektor-gadget/pkg/datasource"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/operators"
)

const defaultTraceDNSImage = "ghcr.io/inspektor-gadget/gadget/trace_dns:v0.52.0"

type TraceDNSRunner struct {
	cfg    GadgetConfig
	client *Client
	log    *slog.Logger
}

func NewTraceDNSRunner(cfg GadgetConfig, client *Client, log *slog.Logger) *TraceDNSRunner {
	if cfg.IGAddress == "" {
		cfg.IGAddress = "tcp://127.0.0.1:8080"
	}
	if cfg.GadgetImage == "" {
		cfg.GadgetImage = defaultTraceDNSImage
	}
	if log == nil {
		log = slog.Default()
	}
	return &TraceDNSRunner{cfg: cfg, client: client, log: log}
}

func (r *TraceDNSRunner) Run(ctx context.Context) error {
	const opPriority = 50000
	return runGadget(ctx, r.cfg, r.log, "trace_dns", func(gctx operators.GadgetContext) error {
		for _, ds := range gctx.GetDataSources() {
			fields := traceDNSFields{ds: ds}
			if err := fields.init(); err != nil {
				return err
			}
			ds.Subscribe(func(_ datasource.DataSource, data datasource.Data) error {
				event := MapDnsEvent(fields.extract(data))
				r.client.Enqueue(event)
				return nil
			}, opPriority)
		}
		return nil
	}, nil)
}

type traceDNSFields struct {
	ds                        datasource.DataSource
	namespace, pod, container datasource.FieldAccessor
	srcAddr, srcPort, srcEp   datasource.FieldAccessor
	dstAddr, dstPort, dstEp   datasource.FieldAccessor
	timestamp                 datasource.FieldAccessor
	qr, name, qtype, rcode    datasource.FieldAccessor
	addresses                 datasource.FieldAccessor
	queryID                   datasource.FieldAccessor
}

func (f *traceDNSFields) init() error {
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
	f.qr = f.ds.GetField("qr")
	f.name = f.ds.GetField("name")
	f.qtype = f.ds.GetField("qtype")
	f.rcode = f.ds.GetField("rcode")
	f.addresses = f.ds.GetField("addresses")
	f.queryID = f.ds.GetField("id")
	if f.pod == nil {
		return fmt.Errorf("missing k8s.podName field")
	}
	if f.name == nil {
		return fmt.Errorf("missing name field")
	}
	return nil
}

func (f *traceDNSFields) extract(data datasource.Data) DNSFields {
	out := DNSFields{}
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
	if f.qr != nil {
		out.QR, _ = f.qr.String(data)
	}
	if f.name != nil {
		out.Name, _ = f.name.String(data)
	}
	if f.qtype != nil {
		out.QType, _ = f.qtype.String(data)
	}
	if f.rcode != nil {
		out.RCode, _ = f.rcode.String(data)
	}
	if f.addresses != nil {
		out.Addresses, _ = f.addresses.String(data)
	}
	if f.queryID != nil {
		id, _ := f.queryID.Uint32(data)
		out.QueryID = id
	}
	return out
}

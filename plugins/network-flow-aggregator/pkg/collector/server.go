package collector

import (
	"io"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/dns"
	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/kube"
	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/store"
	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/upstream"
	aggregv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/aggregator/v1"
)

type Server struct {
	aggregv1.UnimplementedAgentIngestServer
	store    *store.Store
	enricher *kube.Enricher
	dnsCache *dns.Cache
	upstream *upstream.Client
	log      *slog.Logger
}

func NewServer(st *store.Store, enricher *kube.Enricher, dnsCache *dns.Cache, upstreamClient *upstream.Client, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{store: st, enricher: enricher, dnsCache: dnsCache, upstream: upstreamClient, log: log}
}

func (s *Server) PushEvents(stream aggregv1.AgentIngest_PushEventsServer) error {
	var total int64
	for {
		batch, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&aggregv1.PushAck{AcceptedEvents: total})
		}
		if err != nil {
			return status.Errorf(codes.Internal, "recv batch: %v", err)
		}

		accepted, enriched := s.store.AppendBatch(batch, s.enrichEvent)
		if s.upstream != nil && len(enriched) > 0 {
			s.upstream.Enqueue(batch.GetNodeName(), batch.GetAgentId(), enriched)
		}
		total += accepted
		s.log.Debug("ingested batch",
			"node", batch.GetNodeName(),
			"agent", batch.GetAgentId(),
			"events", len(batch.GetEvents()),
			"accepted", accepted,
		)
	}
}

func (s *Server) enrichEvent(event *aggregv1.FlowEvent) store.Enrichment {
	srcNs := event.GetSrc().GetNamespace()
	srcPod := event.GetSrc().GetPod()

	var src kube.WorkloadIdentity
	if s.enricher == nil {
		src = kube.WorkloadIdentity{Namespace: srcNs, Kind: "Pod", Name: srcPod}
	} else {
		src = s.enricher.ResolveSrcWorkload(srcNs, srcPod)
	}

	if event.GetProtocol() == aggregv1.Protocol_PROTOCOL_DNS {
		if event.GetEventKind() == aggregv1.FlowEventKind_FLOW_EVENT_KIND_DNS_RESPONSE && s.dnsCache != nil {
			ts := time.Unix(0, event.GetTimestampUnixNano())
			s.dnsCache.RecordResponse(
				srcNs,
				srcPod,
				event.GetDns().GetName(),
				event.GetDns().GetQtype(),
				event.GetDns().GetRcode(),
				event.GetDns().GetAddresses(),
				ts,
			)
		}
		dst := kube.DstIdentity{Addr: event.GetDst().GetAddr()}
		if s.enricher != nil && event.GetDst().GetAddr() != "" {
			dst = s.enricher.ResolveDst(event.GetDst().GetAddr(), event.GetDst().GetPort())
		}
		return store.Enrichment{
			SrcNamespace:    src.Namespace,
			SrcWorkloadKind: src.Kind,
			SrcWorkloadName: src.Name,
			DstNamespace:    dst.Namespace,
			DstKind:         dst.Kind,
			DstName:         dst.Name,
		}
	}

	var dst kube.DstIdentity
	if s.enricher != nil {
		dst = s.enricher.ResolveDst(event.GetDst().GetAddr(), event.GetDst().GetPort())
	} else {
		dst = kube.DstIdentity{Addr: event.GetDst().GetAddr()}
	}
	if dst.Kind == "" && s.dnsCache != nil {
		if hostname, ok := s.dnsCache.Lookup(srcNs, srcPod, event.GetDst().GetAddr()); ok {
			dst = kube.DstIdentity{Kind: "ExternalHostname", Name: hostname, Addr: event.GetDst().GetAddr()}
		}
	}

	return store.Enrichment{
		SrcNamespace:    src.Namespace,
		SrcWorkloadKind: src.Kind,
		SrcWorkloadName: src.Name,
		DstNamespace:    dst.Namespace,
		DstKind:         dst.Kind,
		DstName:         dst.Name,
	}
}

package collector

import (
	"io"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/kube"
	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/store"
	flowv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow/pkg/flow/v1"
)

type Server struct {
	flowv1.UnimplementedFlowIngestServer
	store    *store.Store
	enricher *kube.Enricher
	log      *slog.Logger
}

func NewServer(st *store.Store, enricher *kube.Enricher, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{store: st, enricher: enricher, log: log}
}

func (s *Server) PushEvents(stream flowv1.FlowIngest_PushEventsServer) error {
	var total int64
	for {
		batch, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&flowv1.PushAck{AcceptedEvents: total})
		}
		if err != nil {
			return status.Errorf(codes.Internal, "recv batch: %v", err)
		}

		accepted := s.store.AppendBatch(batch, s.enrichEvent)
		total += accepted
		s.log.Debug("ingested batch",
			"node", batch.GetNodeName(),
			"agent", batch.GetAgentId(),
			"events", len(batch.GetEvents()),
			"accepted", accepted,
		)
	}
}

func (s *Server) enrichEvent(event *flowv1.FlowEvent) store.Enrichment {
	if s.enricher == nil {
		return store.Enrichment{
			SrcNamespace:    event.GetSrc().GetNamespace(),
			SrcWorkloadKind: "Pod",
			SrcWorkloadName: event.GetSrc().GetPod(),
		}
	}

	src := s.enricher.ResolveSrcWorkload(event.GetSrc().GetNamespace(), event.GetSrc().GetPod())
	dst := s.enricher.ResolveDst(event.GetDst().GetAddr(), event.GetDst().GetPort())

	return store.Enrichment{
		SrcNamespace:    src.Namespace,
		SrcWorkloadKind: src.Kind,
		SrcWorkloadName: src.Name,
		DstNamespace:    dst.Namespace,
		DstKind:         dst.Kind,
		DstName:         dst.Name,
	}
}

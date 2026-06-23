//go:build linux

package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/network-flow/pkg/agent"
)

func main() {
	collectorAddr := flag.String("collector-addr", envOr("COLLECTOR_ADDR", "network-flow-aggregator.insights.svc:4317"), "network-flow-aggregator gRPC address")
	igAddr := flag.String("ig-addr", envOr("IG_ADDR", "tcp://127.0.0.1:8080"), "local Inspektor Gadget gRPC address")
	traceTCPImage := flag.String("trace-tcp-image", envOr("TRACE_TCP_IMAGE", ""), "trace_tcp gadget OCI image")
	topTCPImage := flag.String("top-tcp-image", envOr("TOP_TCP_IMAGE", ""), "top_tcp gadget OCI image")
	traceDNSImage := flag.String("trace-dns-image", envOr("TRACE_DNS_IMAGE", ""), "trace_dns gadget OCI image")
	nodeName := flag.String("node-name", envOr("NODE_NAME", os.Getenv("HOSTNAME")), "Kubernetes node name")
	agentID := flag.String("agent-id", envOr("AGENT_ID", os.Getenv("HOSTNAME")), "unique agent identifier")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	client := agent.NewClient(agent.ClientConfig{
		CollectorAddr: *collectorAddr,
		NodeName:      *nodeName,
		AgentID:       *agentID,
		BatchSize:     100,
		FlushInterval: 2 * time.Second,
	}, log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := client.Run(ctx); err != nil && ctx.Err() == nil {
			log.Error("collector client stopped", "err", err)
			stop()
		}
	}()

	traceRunner := agent.NewTraceTCPRunner(agent.GadgetConfig{
		IGAddress:   *igAddr,
		GadgetImage: *traceTCPImage,
	}, client, log)
	topRunner := agent.NewTopTCPRunner(agent.GadgetConfig{
		IGAddress:   *igAddr,
		GadgetImage: *topTCPImage,
	}, client, log)
	dnsRunner := agent.NewTraceDNSRunner(agent.GadgetConfig{
		IGAddress:   *igAddr,
		GadgetImage: *traceDNSImage,
	}, client, log)

	errCh := make(chan error, 3)
	go func() { errCh <- traceRunner.Run(ctx) }()
	go func() { errCh <- topRunner.Run(ctx) }()
	go func() { errCh <- dnsRunner.Run(ctx) }()

	for i := 0; i < 3; i++ {
		if err := <-errCh; err != nil && ctx.Err() == nil {
			log.Error("gadget runner stopped", "err", err)
			stop()
		}
	}

	client.Flush()
	log.Info("agent stopped")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

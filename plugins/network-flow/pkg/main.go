//go:build linux

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
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
	batchSize := flag.Int("batch-size", parseIntEnv("BATCH_SIZE", 1_000), "number of events to batch before flushing")
	maxPendingEvents := flag.Int("max-pending-events", parseIntEnv("MAX_PENDING_EVENTS", 50_000), "maximum pending events before drop-oldest retention")
	flushInterval := flag.Duration("flush-interval", parseDurationEnv("FLUSH_INTERVAL", 15*time.Second), "interval to flush events")
	reconnectBackoffMin := flag.Duration("reconnect-backoff-min", parseDurationEnv("RECONNECT_BACKOFF_MIN", time.Second), "minimum gRPC reconnect backoff")
	reconnectBackoffMax := flag.Duration("reconnect-backoff-max", parseDurationEnv("RECONNECT_BACKOFF_MAX", 30*time.Second), "maximum gRPC reconnect backoff")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	client := agent.NewClient(agent.ClientConfig{
		CollectorAddr:       *collectorAddr,
		NodeName:            *nodeName,
		AgentID:             *agentID,
		BatchSize:           *batchSize,
		MaxPendingEvents:    *maxPendingEvents,
		FlushInterval:       *flushInterval,
		ReconnectBackoffMin: *reconnectBackoffMin,
		ReconnectBackoffMax: *reconnectBackoffMax,
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

func parseIntEnv(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		val, err := strconv.Atoi(v)
		if err != nil {
			panic(fmt.Errorf("invalid %s: %v", key, err))
		}
		return val
	}
	return fallback
}

func parseDurationEnv(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		val, err := time.ParseDuration(v)
		if err != nil {
			panic(fmt.Errorf("invalid %s: %v", key, err))
		}
		return val
	}
	return fallback
}

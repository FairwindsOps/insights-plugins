package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"google.golang.org/grpc"

	aggregv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/aggregator/v1"
	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector"
	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/dns"
	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/kube"
	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/store"
	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/upstream"
)

func main() {
	grpcAddr := flag.String("grpc-addr", envOr("GRPC_ADDR", ":4317"), "gRPC listen address")
	httpAddr := flag.String("http-addr", envOr("HTTP_ADDR", ":8080"), "debug HTTP listen address")
	kubeconfig := flag.String("kubeconfig", envOr("KUBECONFIG", ""), "path to kubeconfig; in-cluster config is used when empty")
	disableKube := flag.Bool("disable-kube", envOr("DISABLE_KUBE", "") == "true", "skip kubernetes enrichment")
	maxEvents := flag.Int("max-events", parseIntEnv("MAX_EVENTS", 100_000), "maximum in-memory flow events")
	maxAge := flag.Duration("max-age", parseDurationEnv("MAX_AGE", 15*time.Minute), "maximum age of retained flow events")
	insightsAddr := flag.String("insights-grpc-addr", envOr("INSIGHTS_GRPC_ADDR", ""), "Insights network flow gRPC address; disabled when empty")
	organization := flag.String("organization", envOr("ORGANIZATION", ""), "Insights organization slug")
	cluster := flag.String("cluster", envOr("CLUSTER", ""), "Insights cluster name")
	authToken := flag.String("auth-token", envOr("AUTH_TOKEN", ""), "Insights cluster auth token")
	upstreamBatchSize := flag.Int("upstream-batch-size", parseIntEnv("UPSTREAM_BATCH_SIZE", 10_000), "Insights upstream send batch size")
	upstreamFlushInterval := flag.Duration("upstream-flush-interval", parseDurationEnv("UPSTREAM_FLUSH_INTERVAL", 10*time.Second), "Insights upstream flush interval")
	reconnectBackoffMin := flag.Duration("reconnect-backoff-min", parseDurationEnv("RECONNECT_BACKOFF_MIN", time.Second), "minimum gRPC reconnect backoff")
	reconnectBackoffMax := flag.Duration("reconnect-backoff-max", parseDurationEnv("RECONNECT_BACKOFF_MAX", 30*time.Second), "maximum gRPC reconnect backoff")
	logLevel := flag.String("log-level", envOr("LOG_LEVEL", "info"), "log level")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLogLevel(*logLevel)}))
	st := store.NewStore(*maxEvents, *maxAge)
	dnsCache := dns.NewCache(*maxAge)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var enricher *kube.Enricher
	if !*disableKube {
		clients, err := kube.NewClients(ctx, *kubeconfig)
		if err != nil {
			log.Warn("kubernetes client unavailable; running without enrichment", "err", err)
		} else {
			enricher, err = kube.NewEnricher(ctx, clients, log)
			if err != nil {
				log.Warn("kubernetes informers unavailable; running without enrichment", "err", err)
			}
		}
	}

	var upstreamClient *upstream.Client
	if *insightsAddr != "" {
		if *organization == "" || *cluster == "" || *authToken == "" {
			log.Error("insights upstream requires organization, cluster, and auth-token")
			os.Exit(1)
		}
		upstreamClient = upstream.NewClient(upstream.Config{
			InsightsAddr:        *insightsAddr,
			Organization:        *organization,
			Cluster:             *cluster,
			AuthToken:           *authToken,
			BatchSize:           *upstreamBatchSize,
			FlushInterval:       *upstreamFlushInterval,
			ReconnectBackoffMin: *reconnectBackoffMin,
			ReconnectBackoffMax: *reconnectBackoffMax,
		}, st, log)
		go func() {
			if err := upstreamClient.Run(ctx); err != nil && ctx.Err() == nil {
				log.Error("insights upstream client stopped", "err", err)
				stop()
			}
		}()
	}

	grpcServer := grpc.NewServer()
	aggregv1.RegisterAgentIngestServer(grpcServer, collector.NewServer(st, enricher, dnsCache, upstreamClient, log))

	lis, err := net.Listen("tcp", *grpcAddr)
	if err != nil {
		log.Error("listen grpc", "addr", *grpcAddr, "err", err)
		os.Exit(1)
	}

	go func() {
		log.Info("flow-collector gRPC listening", "addr", *grpcAddr, "max_events", *maxEvents, "max_age", *maxAge)
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("grpc serve", "err", err)
			stop()
		}
	}()

	mux := http.NewServeMux()
	collector.NewDebugHTTPServer(st).Register(mux)
	httpServer := &http.Server{Addr: *httpAddr, Handler: mux}

	go func() {
		log.Info("flow-collector HTTP listening", "addr", *httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http serve", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	grpcServer.GracefulStop()
	_ = httpServer.Shutdown(shutdownCtx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseDurationEnv(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func parseIntEnv(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	}
	return slog.LevelInfo
}

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

	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector"
	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/kube"
	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/store"
	flowv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow/pkg/flow/v1"
)

func main() {
	grpcAddr := flag.String("grpc-addr", envOr("GRPC_ADDR", ":4317"), "gRPC listen address")
	httpAddr := flag.String("http-addr", envOr("HTTP_ADDR", ":8080"), "debug HTTP listen address")
	kubeconfig := flag.String("kubeconfig", envOr("KUBECONFIG", ""), "path to kubeconfig; in-cluster config is used when empty")
	disableKube := flag.Bool("disable-kube", envOr("DISABLE_KUBE", "") == "true", "skip kubernetes enrichment")
	maxEvents := flag.Int("max-events", parseIntEnv("MAX_EVENTS", 100_000), "maximum in-memory flow events")
	maxAge := flag.Duration("max-age", parseDurationEnv("MAX_AGE", 15*time.Minute), "maximum age of retained flow events")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	st := store.NewStore(*maxEvents, *maxAge)

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

	grpcServer := grpc.NewServer()
	flowv1.RegisterFlowIngestServer(grpcServer, collector.NewServer(st, enricher, log))

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

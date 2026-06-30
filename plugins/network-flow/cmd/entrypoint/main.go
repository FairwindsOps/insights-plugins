//go:build linux

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	igBin := envOr("IG_BIN", "/bin/gadgettracermanager")
	agentBin := envOr("AGENT_BIN", "/usr/local/bin/network-flow")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cmd := exec.CommandContext(ctx, igBin, "-serve")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Error("start IG daemon", "err", err)
		os.Exit(1)
	}

	go func() {
		<-ctx.Done()
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}()

	log.Info("waiting for IG daemon")
	if err := waitForIG(ctx, igBin); err != nil {
		log.Error("IG daemon not ready", "err", err)
		os.Exit(1)
	}

	log.Info("starting agent")
	agent := exec.CommandContext(ctx, agentBin, os.Args[1:]...)
	agent.Stdout = os.Stdout
	agent.Stderr = os.Stderr
	agent.Env = os.Environ()
	if err := agent.Run(); err != nil {
		log.Error("agent exited", "err", err)
		os.Exit(1)
	}
}

func waitForIG(ctx context.Context, igBin string) error {
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		cmd := exec.CommandContext(ctx, igBin, "-liveness")
		if err := cmd.Run(); err == nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("timeout waiting for IG daemon")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

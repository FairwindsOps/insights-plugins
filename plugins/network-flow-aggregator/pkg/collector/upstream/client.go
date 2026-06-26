package upstream

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/store"
	insightsv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/insights/v1"
)

type Config struct {
	InsightsAddr        string
	Organization        string
	Cluster             string
	AuthToken           string
	BatchSize           int
	FlushInterval       time.Duration
	ReconnectBackoffMin time.Duration
	ReconnectBackoffMax time.Duration
}

type Client struct {
	cfg     Config
	store   *store.Store
	log     *slog.Logger
	flushCh chan struct{}

	dropLogMu            sync.Mutex
	dropLogNext          time.Time
	pendingDropLogCount  int64
	pendingDropLogReason string
}

func NewClient(cfg Config, st *store.Store, log *slog.Logger) *Client {
	cfg.InsightsAddr = normalizeGRPCAddr(cfg.InsightsAddr)
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 10_000
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 10 * time.Second
	}
	if cfg.ReconnectBackoffMin <= 0 {
		cfg.ReconnectBackoffMin = time.Second
	}
	if cfg.ReconnectBackoffMax <= 0 {
		cfg.ReconnectBackoffMax = 30 * time.Second
	}
	if log == nil {
		log = slog.Default()
	}
	return &Client{
		cfg:     cfg,
		store:   st,
		log:     log,
		flushCh: make(chan struct{}, 1),
	}
}

func (c *Client) NotifyAppended() {
	if c.store.UnsentCount() >= c.cfg.BatchSize {
		c.signalFlush()
	}
}

func (c *Client) Flush() {
	c.signalFlush()
}

func (c *Client) Run(ctx context.Context) error {
	ticker := time.NewTicker(c.cfg.FlushInterval)
	defer ticker.Stop()

	backoff := c.cfg.ReconnectBackoffMin
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		conn, stream, err := c.dialStream(ctx)
		if err != nil {
			c.log.Warn("connect insights", "addr", c.cfg.InsightsAddr, "err", err, "backoff", backoff)
			if !sleepOrDone(ctx, backoff) {
				return ctx.Err()
			}
			backoff = c.nextBackoff(backoff)
			continue
		}

		c.log.Info("insights stream connected", "addr", c.cfg.InsightsAddr, "organization", c.cfg.Organization, "cluster", c.cfg.Cluster)
		backoff = c.cfg.ReconnectBackoffMin

		if err := c.sendPending(stream); err != nil {
			c.abortConn(conn)
			c.log.Warn("initial upstream send failed", "err", err)
			if !sleepOrDone(ctx, backoff) {
				return ctx.Err()
			}
			backoff = c.nextBackoff(backoff)
			continue
		}

		if err := c.runConnected(ctx, stream, ticker); err != nil {
			if ctx.Err() != nil {
				_ = c.sendPending(stream)
				c.closeConnGracefully(conn, stream)
				return ctx.Err()
			}
			c.abortConn(conn)
			c.log.Warn("insights stream disconnected", "err", err, "backoff", backoff)
			if !sleepOrDone(ctx, backoff) {
				return ctx.Err()
			}
			backoff = c.nextBackoff(backoff)
			continue
		}

		c.closeConnGracefully(conn, stream)
		return nil
	}
}

func (c *Client) runConnected(ctx context.Context, stream insightsv1.NetworkFlowIngest_PushEnrichedEventsClient, ticker *time.Ticker) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := c.sendPending(stream); err != nil {
				return err
			}
		case <-c.flushCh:
			if err := c.sendPending(stream); err != nil {
				return err
			}
		}
	}
}

func (c *Client) dialStream(ctx context.Context) (*grpc.ClientConn, insightsv1.NetworkFlowIngest_PushEnrichedEventsClient, error) {
	conn, err := grpc.NewClient(
		c.cfg.InsightsAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}

	md := metadata.Pairs("authorization", "Bearer "+c.cfg.AuthToken)
	streamCtx := metadata.NewOutgoingContext(ctx, md)

	client := insightsv1.NewNetworkFlowIngestClient(conn)
	stream, err := client.PushEnrichedEvents(streamCtx)
	if err != nil {
		if err := conn.Close(); err != nil {
			c.log.Warn("close insights connection", "err", err)
		}
		return nil, nil, err
	}
	return conn, stream, nil
}

func (c *Client) sendPending(stream insightsv1.NetworkFlowIngest_PushEnrichedEventsClient) error {
	c.logDroppedUnsent()

	for {
		nodeName, agentID, events, ok := c.store.PeekUnsentBatch(c.cfg.BatchSize)
		if !ok {
			return nil
		}
		msg := &insightsv1.EnrichedFlowEventBatch{
			Organization: c.cfg.Organization,
			Cluster:      c.cfg.Cluster,
			NodeName:     nodeName,
			AgentId:      agentID,
			Events:       events,
		}
		if err := stream.Send(msg); err != nil {
			return err
		}
		c.store.AdvanceSendCursor(len(events))
		c.log.Debug("upstream batch sent", "events", len(events), "node", nodeName, "agent", agentID)
	}
}

func (c *Client) logDroppedUnsent() {
	count, reason := c.store.TakeDroppedUnsent()
	if count == 0 {
		return
	}

	c.dropLogMu.Lock()
	c.pendingDropLogCount += count
	if reason != "" {
		c.pendingDropLogReason = reason
	}
	now := time.Now()
	if now.Before(c.dropLogNext) {
		c.dropLogMu.Unlock()
		return
	}
	c.dropLogNext = now.Add(time.Minute)
	dropped := c.pendingDropLogCount
	logReason := c.pendingDropLogReason
	c.pendingDropLogCount = 0
	c.pendingDropLogReason = ""
	c.dropLogMu.Unlock()

	c.log.Warn("unsent flow events dropped by retention",
		"dropped", dropped,
		"unsent_remaining", c.store.UnsentCount(),
		"reason", logReason,
	)
}

func (c *Client) closeConnGracefully(conn *grpc.ClientConn, stream insightsv1.NetworkFlowIngest_PushEnrichedEventsClient) {
	if stream != nil {
		ack, err := stream.CloseAndRecv()
		if err != nil && err != io.EOF {
			c.log.Warn("close insights stream", "err", err)
		} else if ack != nil {
			c.log.Debug("insights stream closed", "accepted", ack.GetAcceptedEvents())
		}
	}
	if conn != nil {
		if err := conn.Close(); err != nil {
			c.log.Warn("close insights connection", "err", err)
		}
	}
}

func (c *Client) abortConn(conn *grpc.ClientConn) {
	if conn != nil {
		if err := conn.Close(); err != nil {
			c.log.Warn("close insights connection", "err", err)
		}
	}
}

func (c *Client) signalFlush() {
	select {
	case c.flushCh <- struct{}{}:
	default:
	}
}

func sleepOrDone(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (c *Client) nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > c.cfg.ReconnectBackoffMax {
		return c.cfg.ReconnectBackoffMax
	}
	return next
}

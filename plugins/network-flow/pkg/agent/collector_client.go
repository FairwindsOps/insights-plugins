package agent

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	aggregv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/aggregator/v1"
)

const defaultMaxPendingEvents = 50_000

type ClientConfig struct {
	CollectorAddr       string
	NodeName            string
	AgentID             string
	BatchSize           int
	MaxPendingEvents    int
	FlushInterval       time.Duration
	ReconnectBackoffMin time.Duration
	ReconnectBackoffMax time.Duration
}

type Client struct {
	cfg     ClientConfig
	log     *slog.Logger
	mu      sync.Mutex
	events  []*aggregv1.FlowEvent
	flushCh chan struct{}

	droppedPending       int64
	sendFailures         int64
	lastDropReason       string
	dropLogMu            sync.Mutex
	dropLogNext          time.Time
	pendingDropLogCount  int64
	pendingDropLogReason string
}

func NewClient(cfg ClientConfig, log *slog.Logger) *Client {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.MaxPendingEvents <= 0 {
		cfg.MaxPendingEvents = defaultMaxPendingEvents
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 2 * time.Second
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
		log:     log,
		flushCh: make(chan struct{}, 1),
	}
}

func (c *Client) PendingCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.events)
}

func (c *Client) DroppedPending() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.droppedPending
}

func (c *Client) SendFailures() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendFailures
}

func (c *Client) Enqueue(event *aggregv1.FlowEvent) {
	if event == nil {
		return
	}
	c.mu.Lock()
	c.events = append(c.events, event)
	c.enforceMaxLocked()
	shouldFlush := len(c.events) >= c.cfg.BatchSize
	c.mu.Unlock()
	if shouldFlush {
		c.signalFlush()
	}
}

func (c *Client) Flush() {
	c.signalFlush()
}

func (c *Client) signalFlush() {
	select {
	case c.flushCh <- struct{}{}:
	default:
	}
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
			c.log.Warn("connect collector", "addr", c.cfg.CollectorAddr, "err", err, "backoff", backoff)
			if !sleepOrDone(ctx, backoff) {
				return ctx.Err()
			}
			backoff = c.nextBackoff(backoff)
			continue
		}

		c.log.Info("collector stream connected", "addr", c.cfg.CollectorAddr, "pending", c.PendingCount())
		backoff = c.cfg.ReconnectBackoffMin

		if err := c.sendPending(stream); err != nil {
			c.abortConn(conn)
			c.log.Warn("initial send failed", "err", err, "send_failures", c.SendFailures())
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
			c.log.Warn("stream disconnected", "err", err, "backoff", backoff, "send_failures", c.SendFailures())
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

func (c *Client) runConnected(ctx context.Context, stream aggregv1.AgentIngest_PushEventsClient, ticker *time.Ticker) error {
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

func (c *Client) dialStream(ctx context.Context) (*grpc.ClientConn, aggregv1.AgentIngest_PushEventsClient, error) {
	conn, err := grpc.NewClient(
		c.cfg.CollectorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}

	client := aggregv1.NewAgentIngestClient(conn)
	stream, err := client.PushEvents(ctx)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	return conn, stream, nil
}

func (c *Client) drainBatchLocked(max int) *aggregv1.FlowEventBatch {
	if len(c.events) == 0 {
		return nil
	}
	if max <= 0 {
		max = c.cfg.BatchSize
	}
	if max > len(c.events) {
		max = len(c.events)
	}
	batch := &aggregv1.FlowEventBatch{
		NodeName: c.cfg.NodeName,
		AgentId:  c.cfg.AgentID,
		Events:   c.events[:max],
	}
	c.events = c.events[max:]
	return batch
}

func (c *Client) enforceMaxLocked() {
	overflow := len(c.events) - c.cfg.MaxPendingEvents
	if overflow <= 0 {
		return
	}
	c.events = c.events[overflow:]
	c.droppedPending += int64(overflow)
	c.lastDropReason = "max_pending_events"
}

func (c *Client) sendPending(stream aggregv1.AgentIngest_PushEventsClient) error {
	c.logDroppedPending()

	for {
		c.mu.Lock()
		batch := c.drainBatchLocked(c.cfg.BatchSize)
		pending := len(c.events)
		c.mu.Unlock()

		if batch == nil || len(batch.GetEvents()) == 0 {
			return nil
		}

		if err := stream.Send(batch); err != nil {
			c.mu.Lock()
			c.sendFailures++
			c.mu.Unlock()
			c.requeue(batch)
			return err
		}

		c.log.Debug("batch sent", "events", len(batch.GetEvents()), "pending", pending)
	}
}

func (c *Client) requeue(batch *aggregv1.FlowEventBatch) {
	if batch == nil || len(batch.GetEvents()) == 0 {
		return
	}
	c.mu.Lock()
	c.events = append(batch.GetEvents(), c.events...)
	c.enforceMaxLocked()
	c.mu.Unlock()
}

func (c *Client) logDroppedPending() {
	c.mu.Lock()
	count := c.droppedPending
	reason := c.lastDropReason
	if count > 0 {
		c.droppedPending = 0
		c.lastDropReason = ""
	}
	pending := len(c.events)
	c.mu.Unlock()

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

	c.log.Warn("pending flow events dropped by retention",
		"dropped", dropped,
		"pending_remaining", pending,
		"reason", logReason,
	)
}

func (c *Client) closeConnGracefully(conn *grpc.ClientConn, stream aggregv1.AgentIngest_PushEventsClient) {
	if stream != nil {
		ack, err := stream.CloseAndRecv()
		if err != nil && err != io.EOF {
			c.log.Warn("close stream", "err", err)
		} else if ack != nil {
			c.log.Debug("stream closed", "accepted", ack.GetAcceptedEvents())
		}
	}
	if conn != nil {
		conn.Close()
	}
}

func (c *Client) abortConn(conn *grpc.ClientConn) {
	if conn != nil {
		conn.Close()
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

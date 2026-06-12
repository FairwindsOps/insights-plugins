package agent

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	flowv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow/pkg/flow/v1"
)

type ClientConfig struct {
	CollectorAddr       string
	NodeName            string
	AgentID             string
	BatchSize           int
	FlushInterval       time.Duration
	ReconnectBackoffMin time.Duration
	ReconnectBackoffMax time.Duration
}

type Client struct {
	cfg     ClientConfig
	log     *slog.Logger
	mu      sync.Mutex
	events  []*flowv1.FlowEvent
	flushCh chan struct{}
}

func NewClient(cfg ClientConfig, log *slog.Logger) *Client {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
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

func (c *Client) Enqueue(event *flowv1.FlowEvent) {
	if event == nil {
		return
	}
	c.mu.Lock()
	c.events = append(c.events, event)
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

		c.log.Info("collector stream connected", "addr", c.cfg.CollectorAddr)
		backoff = c.cfg.ReconnectBackoffMin

		if err := c.sendPending(stream); err != nil {
			c.abortConn(conn)
			c.log.Warn("initial send failed", "err", err)
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
			c.log.Warn("stream disconnected", "err", err, "backoff", backoff)
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

func (c *Client) runConnected(ctx context.Context, stream flowv1.FlowIngest_PushEventsClient, ticker *time.Ticker) error {
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

func (c *Client) dialStream(ctx context.Context) (*grpc.ClientConn, flowv1.FlowIngest_PushEventsClient, error) {
	conn, err := grpc.NewClient(
		c.cfg.CollectorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}

	client := flowv1.NewFlowIngestClient(conn)
	stream, err := client.PushEvents(ctx)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	return conn, stream, nil
}

func (c *Client) drainLocked() *flowv1.FlowEventBatch {
	if len(c.events) == 0 {
		return nil
	}
	batch := &flowv1.FlowEventBatch{
		NodeName: c.cfg.NodeName,
		AgentId:  c.cfg.AgentID,
		Events:   c.events,
	}
	c.events = nil
	return batch
}

func (c *Client) sendPending(stream flowv1.FlowIngest_PushEventsClient) error {
	c.mu.Lock()
	batch := c.drainLocked()
	c.mu.Unlock()

	if batch == nil || len(batch.GetEvents()) == 0 {
		return nil
	}

	if err := stream.Send(batch); err != nil {
		c.requeue(batch)
		return err
	}

	c.log.Debug("batch sent", "events", len(batch.GetEvents()))
	return nil
}

func (c *Client) requeue(batch *flowv1.FlowEventBatch) {
	if batch == nil || len(batch.GetEvents()) == 0 {
		return
	}
	c.mu.Lock()
	c.events = append(batch.GetEvents(), c.events...)
	c.mu.Unlock()
}

func (c *Client) closeConnGracefully(conn *grpc.ClientConn, stream flowv1.FlowIngest_PushEventsClient) {
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

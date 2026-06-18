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

	flowv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow/pkg/flow/v1"
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
	log     *slog.Logger
	mu      sync.Mutex
	pending []*pendingBatch // TODO: bound pending queue; see TODO.md
	flushCh chan struct{}
}

type pendingBatch struct {
	nodeName string
	agentID  string
	events   []*flowv1.EnrichedFlowEvent
}

func NewClient(cfg Config, log *slog.Logger) *Client {
	cfg.InsightsAddr = normalizeGRPCAddr(cfg.InsightsAddr)
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

func (c *Client) Enqueue(nodeName, agentID string, events []*flowv1.EnrichedFlowEvent) {
	if len(events) == 0 {
		return
	}
	c.mu.Lock()
	c.pending = append(c.pending, &pendingBatch{
		nodeName: nodeName,
		agentID:  agentID,
		events:   events,
	})
	shouldFlush := c.pendingEventCountLocked() >= c.cfg.BatchSize
	c.mu.Unlock()
	if shouldFlush {
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

func (c *Client) runConnected(ctx context.Context, stream flowv1.FlowIngest_PushEnrichedEventsClient, ticker *time.Ticker) error {
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

func (c *Client) dialStream(ctx context.Context) (*grpc.ClientConn, flowv1.FlowIngest_PushEnrichedEventsClient, error) {
	conn, err := grpc.NewClient(
		c.cfg.InsightsAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}

	md := metadata.Pairs("authorization", "Bearer "+c.cfg.AuthToken)
	streamCtx := metadata.NewOutgoingContext(ctx, md)

	client := flowv1.NewFlowIngestClient(conn)
	stream, err := client.PushEnrichedEvents(streamCtx)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	return conn, stream, nil
}

func (c *Client) sendPending(stream flowv1.FlowIngest_PushEnrichedEventsClient) error {
	c.mu.Lock()
	batches := c.pending
	c.pending = nil
	c.mu.Unlock()

	for _, batch := range batches {
		if batch == nil || len(batch.events) == 0 {
			continue
		}
		msg := &flowv1.EnrichedFlowEventBatch{
			Organization: c.cfg.Organization,
			Cluster:      c.cfg.Cluster,
			NodeName:     batch.nodeName,
			AgentId:      batch.agentID,
			Events:       batch.events,
		}
		if err := stream.Send(msg); err != nil {
			c.requeueFront(batch)
			return err
		}
		c.log.Info("upstream batch sent", "events", len(batch.events), "node", batch.nodeName, "agent", batch.agentID)
	}
	return nil
}

func (c *Client) requeueFront(batch *pendingBatch) {
	if batch == nil || len(batch.events) == 0 {
		return
	}
	c.mu.Lock()
	c.pending = append([]*pendingBatch{batch}, c.pending...)
	c.mu.Unlock()
}

func (c *Client) closeConnGracefully(conn *grpc.ClientConn, stream flowv1.FlowIngest_PushEnrichedEventsClient) {
	if stream != nil {
		ack, err := stream.CloseAndRecv()
		if err != nil && err != io.EOF {
			c.log.Warn("close insights stream", "err", err)
		} else if ack != nil {
			c.log.Debug("insights stream closed", "accepted", ack.GetAcceptedEvents())
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

func (c *Client) signalFlush() {
	select {
	case c.flushCh <- struct{}{}:
	default:
	}
}

func (c *Client) pendingEventCountLocked() int {
	total := 0
	for _, batch := range c.pending {
		total += len(batch.events)
	}
	return total
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

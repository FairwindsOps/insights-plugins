package upstream

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/collector/store"
	insightsv1 "github.com/fairwindsops/insights-plugins/plugins/network-flow-aggregator/pkg/insights/v1"
)

const (
	statusLogInterval = time.Minute
	slowSendThreshold = 5 * time.Second
)

type Config struct {
	InsightsAddr        string
	Organization        string
	Cluster             string
	AuthToken           string
	TLS                 TLSConfig
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

	statsMu             sync.Mutex
	slowSendLogNext     time.Time
	sentEventsSinceLog  int64
	sentBatchesSinceLog int64
	sessionSentEvents   int64
	sessionSentBatches  int64
	sessionConnectedAt  time.Time
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
			c.log.Warn("connect insights", "addr", c.cfg.InsightsAddr, "err", err, "backoff", backoff, "unsent", c.store.UnsentCount())
			if !sleepOrDone(ctx, backoff) {
				return ctx.Err()
			}
			backoff = c.nextBackoff(backoff)
			continue
		}

		c.beginSession()
		c.log.Info("insights stream connected", "addr", c.cfg.InsightsAddr, "organization", c.cfg.Organization, "cluster", c.cfg.Cluster, "unsent", c.store.UnsentCount())
		backoff = c.cfg.ReconnectBackoffMin

		statusCtx, cancelStatus := context.WithCancel(ctx)
		go c.statusLogLoop(statusCtx)

		if err := c.sendPending(stream); err != nil {
			cancelStatus()
			c.abortConn(conn)
			c.log.Warn("initial upstream send failed", "err", err, "unsent", c.store.UnsentCount(), "session_sent", c.sessionSent())
			if !sleepOrDone(ctx, backoff) {
				return ctx.Err()
			}
			backoff = c.nextBackoff(backoff)
			continue
		}

		if err := c.runConnected(ctx, stream, ticker); err != nil {
			cancelStatus()
			if ctx.Err() != nil {
				_ = c.sendPending(stream)
				c.closeConnGracefully(conn, stream)
				return ctx.Err()
			}
			c.abortConn(conn)
			c.log.Warn("insights stream disconnected",
				"err", err,
				"backoff", backoff,
				"unsent", c.store.UnsentCount(),
				"session_sent", c.sessionSent(),
				"session_batches", c.sessionBatches(),
				"connected_for", c.connectedFor(),
			)
			if !sleepOrDone(ctx, backoff) {
				return ctx.Err()
			}
			backoff = c.nextBackoff(backoff)
			continue
		}

		cancelStatus()
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
	creds, err := transportCredentials(c.cfg.InsightsAddr, c.cfg.TLS)
	if err != nil {
		return nil, nil, err
	}

	conn, err := grpc.NewClient(
		c.cfg.InsightsAddr,
		grpc.WithTransportCredentials(creds),
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
		started := time.Now()
		if err := stream.Send(msg); err != nil {
			return err
		}
		c.noteSlowSend(time.Since(started), len(events), nodeName, agentID)
		c.store.AdvanceSendCursor(len(events))
		c.noteSent(len(events), 1)
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
	c.dropLogNext = now.Add(statusLogInterval)
	dropped := c.pendingDropLogCount
	logReason := c.pendingDropLogReason
	c.pendingDropLogCount = 0
	c.pendingDropLogReason = ""
	c.dropLogMu.Unlock()

	attrs := []any{
		"dropped", dropped,
		"unsent_remaining", c.store.UnsentCount(),
		"buffered", c.store.Count(),
		"reason", logReason,
	}
	if age := c.store.OldestUnsentAge(); age > 0 {
		attrs = append(attrs, "oldest_unsent_age", age)
	}
	c.log.Warn("unsent flow events dropped by retention", attrs...)
}

func (c *Client) statusLogLoop(ctx context.Context) {
	ticker := time.NewTicker(statusLogInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.maybeLogSendProgress()
		}
	}
}

func (c *Client) beginSession() {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	c.sessionSentEvents = 0
	c.sessionSentBatches = 0
	c.sessionConnectedAt = time.Now()
	c.sentEventsSinceLog = 0
	c.sentBatchesSinceLog = 0
}

func (c *Client) noteSent(events, batches int) {
	if events <= 0 && batches <= 0 {
		return
	}
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	c.sentEventsSinceLog += int64(events)
	c.sentBatchesSinceLog += int64(batches)
	c.sessionSentEvents += int64(events)
	c.sessionSentBatches += int64(batches)
}

func (c *Client) maybeLogSendProgress() {
	c.statsMu.Lock()
	events := c.sentEventsSinceLog
	batches := c.sentBatchesSinceLog
	c.sentEventsSinceLog = 0
	c.sentBatchesSinceLog = 0
	c.statsMu.Unlock()

	unsent := c.store.UnsentCount()
	buffered := c.store.Count()
	if events == 0 && batches == 0 && unsent == 0 {
		return
	}

	attrs := []any{
		"events_sent", events,
		"batches_sent", batches,
		"unsent", unsent,
		"buffered", buffered,
	}
	oldest := c.store.OldestUnsentAge()
	if oldest > 0 {
		attrs = append(attrs, "oldest_unsent_age", oldest)
	}

	// Stall signal: backlog present but nothing flushed this interval.
	if events == 0 && unsent > 0 && oldest >= statusLogInterval {
		c.log.Warn("upstream send stalled", attrs...)
		return
	}
	c.log.Info("upstream send progress", attrs...)
}

func (c *Client) noteSlowSend(elapsed time.Duration, events int, nodeName, agentID string) {
	if elapsed < slowSendThreshold {
		return
	}
	c.statsMu.Lock()
	now := time.Now()
	if now.Before(c.slowSendLogNext) {
		c.statsMu.Unlock()
		return
	}
	c.slowSendLogNext = now.Add(statusLogInterval)
	c.statsMu.Unlock()

	c.log.Warn("upstream batch send slow",
		"duration", elapsed,
		"events", events,
		"node", nodeName,
		"agent", agentID,
		"unsent", c.store.UnsentCount(),
	)
}

func (c *Client) sessionSent() int64 {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	return c.sessionSentEvents
}

func (c *Client) sessionBatches() int64 {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	return c.sessionSentBatches
}

func (c *Client) connectedFor() time.Duration {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	if c.sessionConnectedAt.IsZero() {
		return 0
	}
	return time.Since(c.sessionConnectedAt)
}

func (c *Client) closeConnGracefully(conn *grpc.ClientConn, stream insightsv1.NetworkFlowIngest_PushEnrichedEventsClient) {
	if stream != nil {
		ack, err := stream.CloseAndRecv()
		if err != nil && err != io.EOF {
			c.log.Warn("close insights stream",
				"err", err,
				"session_sent", c.sessionSent(),
				"connected_for", c.connectedFor(),
			)
		} else {
			accepted := int64(0)
			if ack != nil {
				accepted = ack.GetAcceptedEvents()
			}
			c.log.Info("insights stream closed",
				"accepted", accepted,
				"session_sent", c.sessionSent(),
				"session_batches", c.sessionBatches(),
				"connected_for", c.connectedFor(),
				"unsent", c.store.UnsentCount(),
			)
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

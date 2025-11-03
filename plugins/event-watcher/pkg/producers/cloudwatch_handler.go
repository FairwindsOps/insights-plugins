package producers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"log/slog"

	"github.com/allegro/bigcache/v3"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/utils"
)

var alreadyProcessedCloudWatchAuditIDs *bigcache.BigCache

func init() {
	var err error
	config := bigcache.DefaultConfig(60 * time.Minute)
	config.HardMaxCacheSize = 256 // 512MB
	alreadyProcessedCloudWatchAuditIDs, err = bigcache.New(context.Background(), config)
	if err != nil {
		slog.Error("Failed to create bigcache", "error", err)
	}
	slog.Info("Bigcache created", "size", alreadyProcessedCloudWatchAuditIDs.Len(), "hard_max_cache_size", config.HardMaxCacheSize)
}

// CloudWatchHandler handles CloudWatch log processing for policy violations
type CloudWatchHandler struct {
	insightsConfig   models.InsightsConfig
	cloudwatchConfig models.CloudWatchConfig
	eventChannel     chan *models.WatchedEvent
	cloudwatchClient *cloudwatchlogs.Client
	stopCh           chan struct{}
}

type CloudWatchUser struct {
	Username string   `json:"username"`
	UID      string   `json:"uid"`
	Groups   []string `json:"groups"`
}

type CloudWatchObjectRef struct {
	Resource        string `json:"resource"`
	Namespace       string `json:"namespace"`
	Name            string `json:"name"`
	UID             string `json:"uid"`
	APIGroup        string `json:"apiGroup"`
	APIVersion      string `json:"apiVersion"`
	ResourceVersion string `json:"resourceVersion"`
	SubResource     string `json:"subResource"`
}

type CloudWatchResponseStatus struct {
	Metadata map[string]interface{} `json:"metadata"`
	Code     int                    `json:"code"`
	Status   string                 `json:"status"`
	Message  string                 `json:"message"`
	Reason   string                 `json:"reason"`
}

// NewCloudWatchHandler creates a new CloudWatch log handler
func NewCloudWatchHandler(insightsConfig models.InsightsConfig, cloudwatchConfig models.CloudWatchConfig, eventChannel chan *models.WatchedEvent) (*CloudWatchHandler, error) {
	// Create AWS config
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cloudwatchConfig.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS config: %w", err)
	}

	// Create CloudWatch Logs client
	cloudwatchClient := cloudwatchlogs.NewFromConfig(cfg)

	return &CloudWatchHandler{
		insightsConfig:   insightsConfig,
		cloudwatchConfig: cloudwatchConfig,
		eventChannel:     eventChannel,
		cloudwatchClient: cloudwatchClient,
		stopCh:           make(chan struct{}),
	}, nil
}

// Start begins processing CloudWatch logs
func (h *CloudWatchHandler) Start(ctx context.Context) error {
	slog.Info("Starting CloudWatch log processing")

	// Parse poll interval
	pollInterval, err := time.ParseDuration(h.cloudwatchConfig.PollInterval)
	if err != nil {
		return fmt.Errorf("invalid poll interval '%s': %w", h.cloudwatchConfig.PollInterval, err)
	}

	// Test initial connection
	if err := h.testConnection(ctx); err != nil {
		slog.Warn("Initial CloudWatch connection test failed, will retry during processing", "error", err)
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	consecutiveErrors := 0
	const maxConsecutiveErrors = 5

	for {
		select {
		case <-ctx.Done():
			slog.Info("CloudWatch handler context cancelled")
			return ctx.Err()
		case <-h.stopCh:
			slog.Info("CloudWatch handler stopped")
			return nil
		case <-ticker.C:
			if err := h.processLogEvents(ctx); err != nil {
				consecutiveErrors++
				slog.Error("Failed to process CloudWatch log events", "error", err, "consecutive_errors", consecutiveErrors)

				// If we have too many consecutive errors, increase the poll interval temporarily
				if consecutiveErrors >= maxConsecutiveErrors {
					slog.Warn("Too many consecutive errors, temporarily increasing poll interval")
					ticker.Stop()
					ticker = time.NewTicker(pollInterval * 2) // Double the interval
					consecutiveErrors = 0                     // Reset counter
				}
			} else {
				// Reset consecutive error counter on success
				if consecutiveErrors > 0 {
					slog.Info("CloudWatch processing recovered, resetting error counter")
					consecutiveErrors = 0
					// Reset to normal poll interval
					ticker.Stop()
					ticker = time.NewTicker(pollInterval)
				}
			}
		}
	}
}

// testConnection tests the CloudWatch connection
func (h *CloudWatchHandler) testConnection(ctx context.Context) error {
	// Try to describe the log group to test connectivity
	input := &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(h.cloudwatchConfig.LogGroupName),
		Limit:              aws.Int32(1),
	}

	_, err := h.cloudwatchClient.DescribeLogGroups(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to connect to CloudWatch: %w", err)
	}

	slog.Info("CloudWatch connection test successful")
	return nil
}

// Stop stops the CloudWatch handler
func (h *CloudWatchHandler) Stop() {
	if h != nil && h.stopCh != nil {
		close(h.stopCh)
	}
}

// processLogEvents processes CloudWatch log events for policy violations
func (h *CloudWatchHandler) processLogEvents(ctx context.Context) error {
	// Get log streams for the log group with retry logic
	streams, err := h.getLogStreamsWithRetry(ctx)
	if err != nil {
		return fmt.Errorf("failed to get log streams after retries: %w", err)
	}

	// Process each log stream
	for _, stream := range streams {
		if err := h.processLogStreamWithRetry(ctx, stream); err != nil {
			slog.Error("Failed to process log stream after retries", "error", err, "stream", *stream.LogStreamName)
		}
	}

	return nil
}

// getLogStreamsWithRetry retrieves log streams with retry logic
func (h *CloudWatchHandler) getLogStreamsWithRetry(ctx context.Context) ([]types.LogStream, error) {
	const maxRetries = 3
	const retryDelay = 2 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		streams, err := h.getLogStreams(ctx)
		if err == nil {
			return streams, nil
		}

		// Check if error is retryable
		if !h.isRetryableError(err) {
			return nil, fmt.Errorf("non-retryable error: %w", err)
		}

		slog.Warn("Failed to get log streams, retrying...",
			"error", err,
			"attempt", attempt,
			"max_retries", maxRetries)

		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryDelay):
				continue
			}
		}
	}

	return nil, fmt.Errorf("failed to get log streams after %d attempts", maxRetries)
}

// getLogStreams retrieves log streams for the configured log group
func (h *CloudWatchHandler) getLogStreams(ctx context.Context) ([]types.LogStream, error) {
	input := &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: aws.String(h.cloudwatchConfig.LogGroupName),
		OrderBy:      types.OrderByLastEventTime,
		Descending:   aws.Bool(true),
		Limit:        aws.Int32(50), // Limit to recent streams
	}

	result, err := h.cloudwatchClient.DescribeLogStreams(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe log streams: %w", err)
	}

	return result.LogStreams, nil
}

// processLogStreamWithRetry processes a log stream with retry logic
func (h *CloudWatchHandler) processLogStreamWithRetry(ctx context.Context, stream types.LogStream) error {
	const maxRetries = 2
	const retryDelay = 1 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := h.processLogStream(ctx, stream)
		if err == nil {
			return nil
		}

		// Check if error is retryable
		if !h.isRetryableError(err) {
			return fmt.Errorf("non-retryable error: %w", err)
		}

		slog.Warn("Failed to process log stream, retrying...",
			"error", err,
			"stream", *stream.LogStreamName,
			"attempt", attempt,
			"max_retries", maxRetries)

		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryDelay):
				continue
			}
		}
	}

	return fmt.Errorf("failed to process log stream after %d attempts", maxRetries)
}

// isRetryableError determines if an error is retryable
func (h *CloudWatchHandler) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Network/connection errors
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "dial") {
		return true
	}

	// AWS service errors that are retryable
	if strings.Contains(errStr, "ThrottlingException") ||
		strings.Contains(errStr, "ServiceUnavailable") ||
		strings.Contains(errStr, "InternalServerError") ||
		strings.Contains(errStr, "TooManyRequestsException") {
		return true
	}

	// Rate limiting errors
	if strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "RateExceeded") {
		return true
	}

	return false
}

// processLogStream processes events from a specific log stream
func (h *CloudWatchHandler) processLogStream(ctx context.Context, stream types.LogStream) error {
	// Only process streams with recent activity (last 5 minutes)
	if stream.LastIngestionTime == nil {
		return nil
	}

	// Get log events from the stream
	input := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(h.cloudwatchConfig.LogGroupName),
		LogStreamName: stream.LogStreamName,
		StartTime:     aws.Int64((time.Now().Add(-5 * time.Minute)).Unix() * 1000), // Last 5 minutes
		Limit:         aws.Int32(int32(h.cloudwatchConfig.BatchSize)),
	}

	// Apply filter pattern if provided
	if h.cloudwatchConfig.FilterPattern != "" {
		// Use FilterLogEvents instead of GetLogEvents for filtering
		filterInput := &cloudwatchlogs.FilterLogEventsInput{
			LogGroupName:  aws.String(h.cloudwatchConfig.LogGroupName),
			FilterPattern: aws.String(h.cloudwatchConfig.FilterPattern),
			StartTime:     aws.Int64((time.Now().Add(-5 * time.Minute)).Unix() * 1000),
			Limit:         aws.Int32(int32(h.cloudwatchConfig.BatchSize)),
		}
		return h.processFilteredLogEvents(ctx, filterInput)
	}

	result, err := h.cloudwatchClient.GetLogEvents(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to get log events: %w", err)
	}

	// Process each log event
	for _, logEvent := range result.Events {
		if err := h.processLogEvent(ctx, *logEvent.Message); err != nil {
			slog.Error("Failed to process log event", "error", err)
		}
	}

	return nil
}

// processFilteredLogEvents processes filtered log events
func (h *CloudWatchHandler) processFilteredLogEvents(ctx context.Context, input *cloudwatchlogs.FilterLogEventsInput) error {
	result, err := h.cloudwatchClient.FilterLogEvents(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to filter log events: %w", err)
	}

	for _, outputLogEvent := range result.Events {
		if err := h.processLogEvent(ctx, *outputLogEvent.Message); err != nil {
			slog.Error("Failed to process output log event", "error", err)
		}
	}

	return nil
}

// processOutputLogEvent processes a single output log event for policy violations
func (h *CloudWatchHandler) processLogEvent(ctx context.Context, message string) error {
	// Parse the log message as JSON
	var auditEvent models.AuditEvent
	if err := json.Unmarshal([]byte(message), &auditEvent); err != nil {
		// Skip non-JSON log entries
		return nil
	}
	if value, err := alreadyProcessedCloudWatchAuditIDs.Get(auditEvent.AuditID); err == nil && value != nil {
		slog.Debug("Audit ID already processed, skipping", "audit_id", auditEvent.AuditID)
		return nil
	}

	// Check if this is a policy violation
	if utils.IsKyvernoPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) ||
		utils.IsValidatingPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) ||
		utils.IsValidatingAdmissionPolicyViolation(auditEvent.ResponseStatus.Code, auditEvent.ResponseStatus.Message) {

		err := alreadyProcessedCloudWatchAuditIDs.Set(auditEvent.AuditID, []byte("true"))
		if err != nil {
			slog.Warn("Failed to set audit ID in bigcache", "error", err, "audit_id", auditEvent.AuditID)
			return nil
		}
		policyViolationEvent := utils.CreateBlockedPolicyViolationEvent(auditEvent)
		if policyViolationEvent != nil {
			slog.Info("Checking if policy violation event is created", "policy_violation_event", policyViolationEvent)
			slog.Info("Creating watched event from policy violation event", "policy_violation_event", policyViolationEvent)
			utils.CreateBlockedWatchedEventFromPolicyViolationEvent(policyViolationEvent, h.eventChannel)
		}
	} else if utils.IsValidatingAdmissionPolicyViolationAuditOnlyAllowEvent(auditEvent.Annotations) {
		err := alreadyProcessedCloudWatchAuditIDs.Set(auditEvent.AuditID, []byte("true"))
		if err != nil {
			slog.Warn("Failed to set audit ID in bigcache", "error", err, "audit_id", auditEvent.AuditID)
			return nil
		}
		auditOnlyAllowEvent := utils.CreateValidatingAdmissionPolicyViolationAuditOnlyAllowEvent(auditEvent)
		slog.Info("Checking if validating admission policy violation audit only allow event is created", "validating_admission_policy_violation_audit_only_allow_event", auditOnlyAllowEvent)
		if auditOnlyAllowEvent != nil {
			slog.Info("Creating watched event from validating admission policy violation audit only allow event", "validating_admission_policy_violation_audit_only_allow_event", auditOnlyAllowEvent)
			utils.CreateAuditOnlyAllowWatchedEventFromValidatingAdmissionPolicyViolation(auditOnlyAllowEvent, h.eventChannel)
		}
	}
	return nil
}

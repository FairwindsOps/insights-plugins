package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/event-watcher/pkg/models"
)

// CloudWatchHandler handles CloudWatch log processing for policy violations
type CloudWatchHandler struct {
	insightsConfig   models.InsightsConfig
	cloudwatchConfig models.CloudWatchConfig
	eventChannel     chan *event.WatchedEvent
	cloudwatchClient *cloudwatchlogs.Client
	stopCh           chan struct{}
}
type CloudWatchAuditEvent struct {
	Kind                     string                   `json:"kind"`
	APIVersion               string                   `json:"apiVersion"`
	Level                    string                   `json:"level"`
	AuditID                  string                   `json:"auditID"`
	Stage                    string                   `json:"stage"`
	RequestURI               string                   `json:"requestURI"`
	Verb                     string                   `json:"verb"`
	User                     CloudWatchUser           `json:"user"`
	SourceIPs                []string                 `json:"sourceIPs"`
	UserAgent                string                   `json:"userAgent"`
	ObjectRef                CloudWatchObjectRef      `json:"objectRef"`
	ResponseStatus           CloudWatchResponseStatus `json:"responseStatus"`
	RequestReceivedTimestamp time.Time                `json:"requestReceivedTimestamp"`
	StageTimestamp           time.Time                `json:"stageTimestamp"`
	Annotations              map[string]string        `json:"annotations"`
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
func NewCloudWatchHandler(insightsConfig models.InsightsConfig, cloudwatchConfig models.CloudWatchConfig, eventChannel chan *event.WatchedEvent) (*CloudWatchHandler, error) {
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
	close(h.stopCh)
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

	lastEventTime := time.Unix(*stream.LastIngestionTime/1000, 0)
	if time.Since(lastEventTime) > 5*time.Minute {
		return nil // Skip old streams
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
		if err := h.processOutputLogEvent(ctx, logEvent); err != nil {
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

	// Process each filtered log event
	for _, logEvent := range result.Events {
		if err := h.processLogEvent(ctx, logEvent); err != nil {
			slog.Error("Failed to process filtered log event", "error", err)
		}
	}

	return nil
}

// processOutputLogEvent processes a single output log event for policy violations
func (h *CloudWatchHandler) processOutputLogEvent(ctx context.Context, logEvent types.OutputLogEvent) error {
	if logEvent.Message == nil {
		return nil
	}

	// Parse the log message as JSON
	var auditEvent CloudWatchAuditEvent
	if err := json.Unmarshal([]byte(*logEvent.Message), &auditEvent); err != nil {
		// Skip non-JSON log entries
		return nil
	}

	// Check if this is a policy violation
	if h.isKyvernoPolicyViolation(auditEvent) {
		violationEvent := h.createPolicyViolationEventFromAuditEvent(auditEvent)
		if violationEvent != nil {
			select {
			case h.eventChannel <- violationEvent:
				slog.Debug("Sent policy violation event to channel",
					"policy_name", violationEvent.Data["policy_name"],
					"resource", fmt.Sprintf("%s/%s", violationEvent.Namespace, violationEvent.Name))
			case <-ctx.Done():
				return ctx.Err()
			default:
				slog.Warn("Event channel full, dropping policy violation event")
			}
		}
	}

	return nil
}

// processLogEvent processes a single filtered log event for policy violations
func (h *CloudWatchHandler) processLogEvent(ctx context.Context, logEvent types.FilteredLogEvent) error {
	if logEvent.Message == nil {
		return nil
	}

	// Parse the log message as JSON
	var auditEvent CloudWatchAuditEvent
	if err := json.Unmarshal([]byte(*logEvent.Message), &auditEvent); err != nil {
		// Skip non-JSON log entries
		return nil
	}

	// Check if this is a ValidatingAdmissionPolicy violation
	if h.isKyvernoPolicyViolation(auditEvent) {
		violationEvent := h.createPolicyViolationEventFromAuditEvent(auditEvent)
		if violationEvent != nil {
			select {
			case h.eventChannel <- violationEvent:
				slog.Debug("Sent policy violation event to channel",
					"policy_name", violationEvent.Data["policy_name"],
					"resource", fmt.Sprintf("%s/%s", violationEvent.Namespace, violationEvent.Name))
			case <-ctx.Done():
				return ctx.Err()
			default:
				slog.Warn("Event channel full, dropping policy violation event")
			}
		}
	}

	return nil
}

func (h *CloudWatchHandler) isKyvernoPolicyViolation(auditEvent CloudWatchAuditEvent) bool {
	message := auditEvent.ResponseStatus.Message
	if auditEvent.ResponseStatus.Code >= 400 && strings.Contains(message, "kyverno") &&
		strings.Contains(message, "blocked due to the following policies") {
		return true
	}
	return false
}

// createPolicyViolationEventFromAuditEvent creates a policy violation event from audit event
func (h *CloudWatchHandler) createPolicyViolationEventFromAuditEvent(auditEvent CloudWatchAuditEvent) *event.WatchedEvent {
	if !h.isKyvernoPolicyViolation(auditEvent) {
		return nil
	}
	objectRef := auditEvent.ObjectRef
	namespace := objectRef.Namespace
	name := objectRef.Name
	resource := objectRef.Resource

	policies := ExtractPoliciesFromMessage(auditEvent.ResponseStatus.Message)
	policyMessage := auditEvent.ResponseStatus.Message

	reason := "Allowed"
	if auditEvent.ResponseStatus.Code >= 400 {
		reason = "Blocked"
	}
	// Create the policy violation event
	violationEvent := &event.WatchedEvent{
		EventType:    event.EventTypeAdded,
		ResourceType: resource,
		Namespace:    namespace,
		Name:         fmt.Sprintf("policy-violation-%s-%s-%s", resource, name, auditEvent.AuditID),
		UID:          auditEvent.AuditID,
		Timestamp:    auditEvent.StageTimestamp.Unix(),
		EventTime:    auditEvent.StageTimestamp.UTC().Format(time.RFC3339),
		Data: map[string]interface{}{
			"reason":  reason,
			"message": policyMessage,
			"blocked": reason == "Blocked",
			"source": map[string]interface{}{
				"component": "cloudwatch",
			},
			"involvedObject": objectRef,
			"firstTimestamp": auditEvent.RequestReceivedTimestamp.Format(time.RFC3339),
			"lastTimestamp":  auditEvent.StageTimestamp.Format(time.RFC3339),
			"metadata":       auditEvent.Annotations,
		},
		Metadata: map[string]interface{}{
			"audit_id":      auditEvent.AuditID,
			"policies":      policies,
			"resource_name": name,
			"namespace":     namespace,
			"action":        reason,
			"message":       policyMessage,
			"timestamp":     auditEvent.StageTimestamp.Format(time.RFC3339),
			"event_time":    auditEvent.StageTimestamp.Format(time.RFC3339),
		},
	}
	return violationEvent
}

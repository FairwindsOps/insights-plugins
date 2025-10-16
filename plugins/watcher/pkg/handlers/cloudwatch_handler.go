package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/sirupsen/logrus"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
)

// CloudWatchHandler handles CloudWatch log processing for policy violations
type CloudWatchHandler struct {
	insightsConfig   models.InsightsConfig
	cloudwatchConfig models.CloudWatchConfig
	eventChannel     chan *event.WatchedEvent
	cloudwatchClient *cloudwatchlogs.Client
	stopCh           chan struct{}
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
	logrus.Info("Starting CloudWatch log processing")

	// Parse poll interval
	pollInterval, err := time.ParseDuration(h.cloudwatchConfig.PollInterval)
	if err != nil {
		return fmt.Errorf("invalid poll interval '%s': %w", h.cloudwatchConfig.PollInterval, err)
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logrus.Info("CloudWatch handler context cancelled")
			return ctx.Err()
		case <-h.stopCh:
			logrus.Info("CloudWatch handler stopped")
			return nil
		case <-ticker.C:
			if err := h.processLogEvents(ctx); err != nil {
				logrus.WithError(err).Error("Failed to process CloudWatch log events")
			}
		}
	}
}

// Stop stops the CloudWatch handler
func (h *CloudWatchHandler) Stop() {
	close(h.stopCh)
}

// processLogEvents processes CloudWatch log events for policy violations
func (h *CloudWatchHandler) processLogEvents(ctx context.Context) error {
	// Get log streams for the log group
	streams, err := h.getLogStreams(ctx)
	if err != nil {
		return fmt.Errorf("failed to get log streams: %w", err)
	}

	// Process each log stream
	for _, stream := range streams {
		if err := h.processLogStream(ctx, stream); err != nil {
			logrus.WithError(err).WithField("stream", *stream.LogStreamName).Error("Failed to process log stream")
		}
	}

	return nil
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
			logrus.WithError(err).Error("Failed to process log event")
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
			logrus.WithError(err).Error("Failed to process filtered log event")
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
	var auditEvent map[string]interface{}
	if err := json.Unmarshal([]byte(*logEvent.Message), &auditEvent); err != nil {
		// Skip non-JSON log entries
		return nil
	}

	// Check if this is a ValidatingAdmissionPolicy violation
	if h.isValidatingAdmissionPolicyViolation(auditEvent) {
		violationEvent := h.createPolicyViolationEventFromOutput(auditEvent, logEvent)
		if violationEvent != nil {
			select {
			case h.eventChannel <- violationEvent:
				logrus.WithFields(logrus.Fields{
					"policy_name": violationEvent.Data["policy_name"],
					"resource":    fmt.Sprintf("%s/%s", violationEvent.Namespace, violationEvent.Name),
				}).Debug("Sent policy violation event to channel")
			case <-ctx.Done():
				return ctx.Err()
			default:
				logrus.Warn("Event channel full, dropping policy violation event")
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
	var auditEvent map[string]interface{}
	if err := json.Unmarshal([]byte(*logEvent.Message), &auditEvent); err != nil {
		// Skip non-JSON log entries
		return nil
	}

	// Check if this is a ValidatingAdmissionPolicy violation
	if h.isValidatingAdmissionPolicyViolation(auditEvent) {
		violationEvent := h.createPolicyViolationEvent(auditEvent, logEvent)
		if violationEvent != nil {
			select {
			case h.eventChannel <- violationEvent:
				logrus.WithFields(logrus.Fields{
					"policy_name": violationEvent.Data["policy_name"],
					"resource":    fmt.Sprintf("%s/%s", violationEvent.Namespace, violationEvent.Name),
				}).Debug("Sent policy violation event to channel")
			case <-ctx.Done():
				return ctx.Err()
			default:
				logrus.Warn("Event channel full, dropping policy violation event")
			}
		}
	}

	return nil
}

// isValidatingAdmissionPolicyViolation checks if the audit event represents a VAP violation
func (h *CloudWatchHandler) isValidatingAdmissionPolicyViolation(auditEvent map[string]interface{}) bool {
	// Check if this is an admission controller response
	stage, ok := auditEvent["stage"].(string)
	if !ok || stage != "ResponseComplete" {
		return false
	}

	// Check if the response was blocked (status code >= 400)
	responseStatus, ok := auditEvent["responseStatus"].(map[string]interface{})
	if !ok {
		return false
	}

	code, ok := responseStatus["code"].(float64)
	if !ok || code < 400 {
		return false
	}

	// Check if this is related to ValidatingAdmissionPolicy
	annotations, ok := auditEvent["annotations"].(map[string]interface{})
	if !ok {
		return false
	}

	// Look for VAP-related annotations
	for key, value := range annotations {
		if strings.Contains(strings.ToLower(key), "validatingadmissionpolicy") ||
			strings.Contains(strings.ToLower(fmt.Sprintf("%v", value)), "validatingadmissionpolicy") {
			return true
		}
	}

	return false
}

// createPolicyViolationEventFromOutput creates a policy violation event from output log event
func (h *CloudWatchHandler) createPolicyViolationEventFromOutput(auditEvent map[string]interface{}, logEvent types.OutputLogEvent) *event.WatchedEvent {
	// Extract basic information
	request, ok := auditEvent["requestURI"].(string)
	if !ok {
		return nil
	}

	// Extract resource information
	objectRef, ok := auditEvent["objectRef"].(map[string]interface{})
	if !ok {
		return nil
	}

	namespace, _ := objectRef["namespace"].(string)
	name, _ := objectRef["name"].(string)
	resource, _ := objectRef["resource"].(string)
	apiVersion, _ := objectRef["apiVersion"].(string)

	// Extract policy information from annotations
	annotations, ok := auditEvent["annotations"].(map[string]interface{})
	if !ok {
		return nil
	}

	policyName := "Unknown"
	policyMessage := "Policy violation detected"

	// Try to extract policy name and message from annotations
	for key, value := range annotations {
		if strings.Contains(strings.ToLower(key), "validatingadmissionpolicy") {
			policyName = fmt.Sprintf("%v", value)
		}
		if strings.Contains(strings.ToLower(key), "message") {
			policyMessage = fmt.Sprintf("%v", value)
		}
	}

	// Extract timestamp
	timestamp := time.Now().Unix()
	if logEvent.Timestamp != nil {
		timestamp = *logEvent.Timestamp / 1000
	}

	// Create the policy violation event
	violationEvent := &event.WatchedEvent{
		EventType:    "PolicyViolation",
		ResourceType: resource,
		Namespace:    namespace,
		Name:         name,
		UID:          fmt.Sprintf("cloudwatch-%d", timestamp),
		Timestamp:    timestamp,
		Data: map[string]interface{}{
			"reason":        "PolicyViolation",
			"type":          "Warning",
			"message":       fmt.Sprintf("policy %s fail: %s", policyName, policyMessage),
			"policy_name":   policyName,
			"policy_result": "Block",
			"blocked":       true,
			"source":        "cloudwatch",
			"request_uri":   request,
			"api_version":   apiVersion,
		},
		Metadata: map[string]interface{}{
			"log_group":  h.cloudwatchConfig.LogGroupName,
			"aws_region": h.cloudwatchConfig.Region,
		},
	}

	return violationEvent
}

// createPolicyViolationEvent creates a policy violation event from audit log data
func (h *CloudWatchHandler) createPolicyViolationEvent(auditEvent map[string]interface{}, logEvent types.FilteredLogEvent) *event.WatchedEvent {
	// Extract basic information
	request, ok := auditEvent["requestURI"].(string)
	if !ok {
		return nil
	}

	// Extract resource information
	objectRef, ok := auditEvent["objectRef"].(map[string]interface{})
	if !ok {
		return nil
	}

	namespace, _ := objectRef["namespace"].(string)
	name, _ := objectRef["name"].(string)
	resource, _ := objectRef["resource"].(string)
	apiVersion, _ := objectRef["apiVersion"].(string)

	// Extract policy information from annotations
	annotations, ok := auditEvent["annotations"].(map[string]interface{})
	if !ok {
		return nil
	}

	policyName := "Unknown"
	policyMessage := "Policy violation detected"

	// Try to extract policy name and message from annotations
	for key, value := range annotations {
		if strings.Contains(strings.ToLower(key), "validatingadmissionpolicy") {
			policyName = fmt.Sprintf("%v", value)
		}
		if strings.Contains(strings.ToLower(key), "message") {
			policyMessage = fmt.Sprintf("%v", value)
		}
	}

	// Extract timestamp
	timestamp := time.Now().Unix()
	if logEvent.Timestamp != nil {
		timestamp = *logEvent.Timestamp / 1000
	}

	// Create the policy violation event
	violationEvent := &event.WatchedEvent{
		EventType:    "PolicyViolation",
		ResourceType: resource,
		Namespace:    namespace,
		Name:         name,
		UID:          fmt.Sprintf("cloudwatch-%d", timestamp),
		Timestamp:    timestamp,
		Data: map[string]interface{}{
			"reason":        "PolicyViolation",
			"type":          "Warning",
			"message":       fmt.Sprintf("policy %s fail: %s", policyName, policyMessage),
			"policy_name":   policyName,
			"policy_result": "Block",
			"blocked":       true,
			"source":        "cloudwatch",
			"request_uri":   request,
			"api_version":   apiVersion,
		},
		Metadata: map[string]interface{}{
			"log_group":  h.cloudwatchConfig.LogGroupName,
			"log_stream": logEvent.LogStreamName,
			"event_id":   logEvent.EventId,
			"aws_region": h.cloudwatchConfig.Region,
		},
	}

	return violationEvent
}

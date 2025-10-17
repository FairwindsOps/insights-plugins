package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

func main() {
	var (
		logGroupName = flag.String("log-group", "/aws/eks/staging-eks/cluster", "CloudWatch log group name")
		region       = flag.String("region", "us-east-1", "AWS region")
		timeout      = flag.Duration("timeout", 30*time.Second, "Test timeout")
	)
	flag.Parse()

	fmt.Printf("🔍 Testing CloudWatch access...\n")
	fmt.Printf("   Log Group: %s\n", *logGroupName)
	fmt.Printf("   Region: %s\n", *region)
	fmt.Printf("   Timeout: %s\n", *timeout)
	fmt.Println()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Load AWS config
	fmt.Println("📡 Loading AWS configuration...")
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(*region))
	if err != nil {
		log.Fatalf("❌ Failed to load AWS config: %v", err)
	}
	fmt.Println("✅ AWS configuration loaded successfully")

	// Create CloudWatch Logs client
	fmt.Println("🔗 Creating CloudWatch Logs client...")
	client := cloudwatchlogs.NewFromConfig(cfg)
	fmt.Println("✅ CloudWatch Logs client created")

	// Test 1: Describe log groups
	fmt.Println("\n🧪 Test 1: Describe log groups...")
	describeInput := &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(*logGroupName),
		Limit:              aws.Int32(5),
	}

	describeResult, err := client.DescribeLogGroups(ctx, describeInput)
	if err != nil {
		log.Fatalf("❌ Failed to describe log groups: %v", err)
	}

	if len(describeResult.LogGroups) == 0 {
		fmt.Printf("⚠️  No log groups found with prefix: %s\n", *logGroupName)
	} else {
		fmt.Printf("✅ Found %d log group(s):\n", len(describeResult.LogGroups))
		for _, lg := range describeResult.LogGroups {
			created := "unknown"
			if lg.CreationTime != nil {
				created = time.Unix(*lg.CreationTime/1000, 0).Format(time.RFC3339)
			}
			fmt.Printf("   - %s (created: %s)\n", *lg.LogGroupName, created)
		}
	}

	// Test 2: Describe log streams
	fmt.Println("\n🧪 Test 2: Describe log streams...")
	streamsInput := &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: aws.String(*logGroupName),
		OrderBy:      types.OrderByLastEventTime,
		Descending:   aws.Bool(true),
		Limit:        aws.Int32(10),
	}

	streamsResult, err := client.DescribeLogStreams(ctx, streamsInput)
	if err != nil {
		log.Fatalf("❌ Failed to describe log streams: %v", err)
	}

	if len(streamsResult.LogStreams) == 0 {
		fmt.Printf("⚠️  No log streams found in log group: %s\n", *logGroupName)
	} else {
		fmt.Printf("✅ Found %d log stream(s):\n", len(streamsResult.LogStreams))
		for i, stream := range streamsResult.LogStreams {
			if i >= 3 { // Show only first 3 streams
				fmt.Printf("   ... and %d more streams\n", len(streamsResult.LogStreams)-3)
				break
			}
			lastEvent := "never"
			if stream.LastIngestionTime != nil {
				lastEvent = time.Unix(*stream.LastIngestionTime/1000, 0).Format(time.RFC3339)
			}
			fmt.Printf("   - %s (last event: %s)\n", *stream.LogStreamName, lastEvent)
		}
	}

	// Test 3: Get recent log events
	fmt.Println("\n🧪 Test 3: Get recent log events...")
	if len(streamsResult.LogStreams) > 0 {
		stream := streamsResult.LogStreams[0]
		eventsInput := &cloudwatchlogs.GetLogEventsInput{
			LogGroupName:  aws.String(*logGroupName),
			LogStreamName: stream.LogStreamName,
			StartTime:     aws.Int64((time.Now().Add(-5 * time.Minute)).Unix() * 1000),
			Limit:         aws.Int32(5),
		}

		eventsResult, err := client.GetLogEvents(ctx, eventsInput)
		if err != nil {
			log.Fatalf("❌ Failed to get log events: %v", err)
		}

		if len(eventsResult.Events) == 0 {
			fmt.Printf("⚠️  No recent events found in stream: %s\n", *stream.LogStreamName)
		} else {
			fmt.Printf("✅ Found %d recent event(s) in stream %s:\n", 
				len(eventsResult.Events), *stream.LogStreamName)
			for i, event := range eventsResult.Events {
				if i >= 2 { // Show only first 2 events
					fmt.Printf("   ... and %d more events\n", len(eventsResult.Events)-2)
					break
				}
				timestamp := time.Unix(*event.Timestamp/1000, 0).Format(time.RFC3339)
				message := *event.Message
				if len(message) > 100 {
					message = message[:100] + "..."
				}
				fmt.Printf("   [%s] %s\n", timestamp, message)
			}
		}
	} else {
		fmt.Println("⚠️  Skipping log events test - no streams available")
	}

	// Test 4: Filter log events (if we have events)
	fmt.Println("\n🧪 Test 4: Filter log events for policy violations...")
	filterInput := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:  aws.String(*logGroupName),
		FilterPattern: aws.String("{ $.stage = \"ResponseComplete\" && $.responseStatus.code >= 400 }"),
		StartTime:     aws.Int64((time.Now().Add(-10 * time.Minute)).Unix() * 1000),
		Limit:         aws.Int32(5),
	}

	filterResult, err := client.FilterLogEvents(ctx, filterInput)
	if err != nil {
		log.Fatalf("❌ Failed to filter log events: %v", err)
	}

	if len(filterResult.Events) == 0 {
		fmt.Println("ℹ️  No policy violation events found in the last 10 minutes")
	} else {
		fmt.Printf("🚨 Found %d potential policy violation event(s):\n", len(filterResult.Events))
		for i, event := range filterResult.Events {
			if i >= 2 { // Show only first 2 events
				fmt.Printf("   ... and %d more events\n", len(filterResult.Events)-2)
				break
			}
			timestamp := time.Unix(*event.Timestamp/1000, 0).Format(time.RFC3339)
			message := *event.Message
			if len(message) > 150 {
				message = message[:150] + "..."
			}
			fmt.Printf("   [%s] %s\n", timestamp, message)
		}
	}

	fmt.Println("\n🎉 CloudWatch access test completed successfully!")
	fmt.Println("\n📋 Summary:")
	fmt.Println("   ✅ AWS authentication working")
	fmt.Println("   ✅ CloudWatch Logs API access working")
	fmt.Println("   ✅ Log group access confirmed")
	fmt.Println("   ✅ Log stream enumeration working")
	fmt.Println("   ✅ Log event retrieval working")
	fmt.Println("   ✅ Filter pattern matching working")
	
	fmt.Println("\n🚀 Ready to run the full watcher with CloudWatch integration!")
}

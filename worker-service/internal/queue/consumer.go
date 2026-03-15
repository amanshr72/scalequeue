package queue

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
	"worker-service/internal/processor"

	"github.com/redis/go-redis/v9"
)

type Consumer struct {
	client        *redis.Client
	streamName    string
	consumerGroup string
	consumerName  string              // unique per worker instance
	worker        processor.Processor // ← interface, not *worker.Worker
	db            *sql.DB
}

func NewConsumer(
	redisURL string,
	streamName string,
	consumerGroup string,
	consumerName string,
	w processor.Processor,
	database *sql.DB,
) *Consumer {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("[Consumer] Invalid Redis URL: %v", err)
	}

	client := redis.NewClient(opt)

	// Verify connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("[Consumer] Failed to connect to Redis: %v", err)
	}
	log.Printf("[Consumer %s] Redis connected", consumerName)

	return &Consumer{
		client:        client,
		streamName:    streamName,
		consumerGroup: consumerGroup,
		consumerName:  consumerName,
		worker:        w,
		db:            database,
	}
}

// EnsureConsumerGroup creates the consumer group if it doesn't exist
// MKSTREAM creates the stream itself if it doesn't exist yet
func (c *Consumer) EnsureConsumerGroup(ctx context.Context) error {
	err := c.client.XGroupCreateMkStream(
		ctx,
		c.streamName,
		c.consumerGroup,
		"0", // start reading from beginning of stream
	).Err()

	if err != nil && err.Error() == "BUSYGROUP Consumer Group name already exists" {
		// Already exists — this is fine, not an error
		log.Printf("[Consumer %s] Consumer group already exists", c.consumerName)
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	log.Printf("[Consumer %s] Consumer group '%s' ready", c.consumerName, c.consumerGroup)
	return nil
}

// Start begins the main consume loop
func (c *Consumer) Start(ctx context.Context) {
	log.Printf("[Consumer %s] Starting consume loop", c.consumerName)

	// First — recover any stuck messages from previous runs (PEL recovery)
	c.recoverPendingMessages(ctx)

	// Main loop
	for {
		select {
		case <-ctx.Done():
			log.Printf("[Consumer %s] Shutting down", c.consumerName)
			return
		default:
			c.readAndProcess(ctx)
		}
	}
}

// readAndProcess pulls one batch of messages and processes them
func (c *Consumer) readAndProcess(ctx context.Context) {
	// XREADGROUP: pull up to 1 message at a time
	// ">" means only new messages not yet delivered to any consumer
	// BlockMilliseconds: wait up to 2s for new messages before returning empty
	result, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.consumerGroup,
		Consumer: c.consumerName,
		Streams:  []string{c.streamName, ">"},
		Count:    1,
		Block:    2000 * time.Millisecond,
	}).Result()

	if err != nil {
		if err == redis.Nil || err.Error() == "redis: nil" {
			// No messages available — loop again
			return
		}
		if strings.Contains(err.Error(), "context canceled") {
			return
		}
		log.Printf("[Consumer %s] XReadGroup error: %v", c.consumerName, err)
		time.Sleep(1 * time.Second)
		return
	}

	for _, stream := range result {
		for _, msg := range stream.Messages {
			c.handleMessage(ctx, msg)
		}
	}
}

// handleMessage processes a single Redis Stream message
func (c *Consumer) handleMessage(ctx context.Context, msg redis.XMessage) {
	jobID, ok := msg.Values["job_id"].(string)
	if !ok || jobID == "" {
		log.Printf("[Consumer %s] Message %s has no job_id, acking and skipping", c.consumerName, msg.ID)
		c.ack(ctx, msg.ID) // ack bad messages so they don't block forever
		return
	}

	err := c.worker.ProcessJob(ctx, jobID, msg.ID)
	if err != nil {
		log.Printf("Unhandled error for job %s: %v", jobID, err)
	}
	c.ack(ctx, msg.ID)
}

// ack acknowledges a message — removes it from Pending Entry List
func (c *Consumer) ack(ctx context.Context, msgID string) {
	if err := c.client.XAck(ctx, c.streamName, c.consumerGroup, msgID).Err(); err != nil {
		log.Printf("[Consumer %s] Failed to ACK message %s: %v", c.consumerName, msgID, err)
	}
}

// recoverPendingMessages handles the PEL — messages delivered but never ACKed
// This happens when a worker crashes mid-processing
func (c *Consumer) recoverPendingMessages(ctx context.Context) {
	log.Printf("[Consumer %s] Checking for pending messages (PEL recovery)...", c.consumerName)

	// XPENDING: list messages that were delivered but not ACKed
	// "-" and "+" mean min and max ID (all pending)
	pending, err := c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: c.streamName,
		Group:  c.consumerGroup,
		Start:  "-",
		End:    "+",
		Count:  100,
	}).Result()

	if err != nil {
		log.Printf("[Consumer %s] Failed to check pending messages: %v", c.consumerName, err)
		return
	}

	if len(pending) == 0 {
		log.Printf("[Consumer %s] No pending messages found", c.consumerName)
		return
	}

	log.Printf("[Consumer %s] Found %d pending message(s) — recovering...", c.consumerName, len(pending))

	for _, p := range pending {
		// Only recover messages stuck for more than 30 seconds
		// Avoids stealing messages from a worker that's still alive and processing
		stuckThreshold := 30 * time.Second
		if p.Idle < stuckThreshold {
			log.Printf("[Consumer %s] Message %s idle for %s — still within threshold, skipping",
				c.consumerName, p.ID, p.Idle)
			continue
		}

		// XCLAIM: take ownership of the stuck message
		claimed, err := c.client.XClaim(ctx, &redis.XClaimArgs{
			Stream:   c.streamName,
			Group:    c.consumerGroup,
			Consumer: c.consumerName,
			MinIdle:  stuckThreshold,
			Messages: []string{p.ID},
		}).Result()

		if err != nil {
			log.Printf("[Consumer %s] Failed to claim message %s: %v", c.consumerName, p.ID, err)
			continue
		}

		for _, msg := range claimed {
			log.Printf("[Consumer %s] Recovering stuck message %s (was idle %s)",
				c.consumerName, msg.ID, p.Idle)
			c.handleMessage(ctx, msg)
		}
	}
}

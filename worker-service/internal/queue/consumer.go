package queue

import (
	"context"
	"errors"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"strconv"
	"time"
	"worker-service/internal/metrics"
	"worker-service/internal/processor"
)

type Consumer struct {
	Client                                     redis.UniversalClient
	Stream, Group, Name                        string
	Worker                                     processor.Processor
	RecoveryInterval, RecoveryIdle, JobTimeout time.Duration
	cursor                                     string
}

func (c *Consumer) EnsureConsumerGroup(ctx context.Context) error {
	err := c.Client.XGroupCreateMkStream(ctx, c.Stream, c.Group, "0").Err()
	if err != nil && err.Error() == "BUSYGROUP Consumer Group name already exists" {
		return nil
	}
	return err
}
func (c *Consumer) Start(stop, execution context.Context) {
	next := time.Now()
	c.cursor = "0-0"
	for stop.Err() == nil {
		if !time.Now().Before(next) {
			c.recover(stop, execution)
			next = time.Now().Add(c.RecoveryInterval)
		}
		if stop.Err() != nil {
			return
		}
		streams, err := c.Client.XReadGroup(stop, &redis.XReadGroupArgs{Group: c.Group, Consumer: c.Name, Streams: []string{c.Stream, ">"}, Count: 1, Block: time.Second}).Result()
		if err != nil {
			if stop.Err() != nil {
				return
			}
			if !errors.Is(err, redis.Nil) {
				slog.Error("stream read failed", "error", err)
				select {
				case <-stop.Done():
					return
				case <-time.After(time.Second):
				}
			}
			continue
		}
		for _, s := range streams {
			for _, m := range s.Messages {
				if stop.Err() != nil {
					return
				}
				c.handle(execution, m)
			}
		}
	}
}
func (c *Consumer) handle(ctx context.Context, m redis.XMessage) {
	id, _ := m.Values["job_id"].(string)
	g, _ := m.Values["generation"].(string)
	generation, err := strconv.Atoi(g)
	if id == "" || err != nil || generation < 1 {
		// Preserve malformed deliveries for investigation rather than silently discard.
		slog.Error("malformed delivery retained", "message_id", m.ID)
		metrics.Errors.Add(1)
		return
	}
	task, cancel := context.WithTimeout(ctx, c.JobTimeout)
	defer cancel()
	if err = c.Worker.ProcessJob(task, id, generation); err != nil {
		slog.Warn("delivery retained for recovery", "job_id", id, "error", err)
		metrics.Errors.Add(1)
		return
	}
	if err = c.Client.XAck(ctx, c.Stream, c.Group, m.ID).Err(); err != nil {
		slog.Error("ack failed", "job_id", id, "error", err)
		metrics.Errors.Add(1)
	}
}
func (c *Consumer) recover(stop, ctx context.Context) {
	messages, next, err := c.Client.XAutoClaim(ctx, &redis.XAutoClaimArgs{Stream: c.Stream, Group: c.Group, Consumer: c.Name, MinIdle: c.RecoveryIdle, Start: c.cursor, Count: 100}).Result()
	if err != nil {
		slog.Warn("recovery scan failed", "error", err)
		return
	}
	c.cursor = next
	for _, m := range messages {
		if ctx.Err() != nil || stop.Err() != nil {
			return
		}
		c.handle(ctx, m)
	}
}

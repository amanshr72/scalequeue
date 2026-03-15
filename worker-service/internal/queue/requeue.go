package queue

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

type Requeuer struct {
	client     *redis.Client
	streamName string
}

func NewRequeuer(redisURL string, streamName string) *Requeuer {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("[Requeuer] Invalid Redis URL: %v", err)
	}
	return &Requeuer{
		client:     redis.NewClient(opt),
		streamName: streamName,
	}
}

func (r *Requeuer) Enqueue(ctx context.Context, jobID string, jobType string) error {
	msgID, err := r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: r.streamName,
		Values: map[string]interface{}{
			"job_id":   jobID,
			"job_type": jobType,
		},
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to enqueue job %s: %w", jobID, err)
	}

	log.Printf("[Requeuer] Job %s enqueued with message ID %s", jobID, msgID)
	return nil
}

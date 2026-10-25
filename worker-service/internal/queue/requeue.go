package queue

import (
	"context"
	"github.com/redis/go-redis/v9"
)

func Publish(client redis.UniversalClient, stream string) func(context.Context, string, int) error {
	return func(ctx context.Context, id string, generation int) error {
		return client.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: map[string]interface{}{"job_id": id, "generation": generation}}).Err()
	}
}

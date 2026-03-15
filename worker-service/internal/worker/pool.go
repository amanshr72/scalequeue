package worker

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"

	"worker-service/internal/handlers"
	"worker-service/internal/queue"
)

type Pool struct {
	size          int
	db            *sql.DB
	registry      *handlers.Registry
	requeuer      *queue.Requeuer
	redisURL      string
	streamName    string
	consumerGroup string
}

func NewPool(
	size int,
	db *sql.DB,
	registry *handlers.Registry,
	redisURL string,
	streamName string,
	consumerGroup string,
) *Pool {
	requeuer := queue.NewRequeuer(redisURL, streamName)
	return &Pool{
		size:          size,
		db:            db,
		registry:      registry,
		requeuer:      requeuer,
		redisURL:      redisURL,
		streamName:    streamName,
		consumerGroup: consumerGroup,
	}
}

// Start launches N workers, each in its own goroutine
func (p *Pool) Start(ctx context.Context) {
	log.Printf("[Pool] Starting %d workers", p.size)

	var wg sync.WaitGroup

	for i := 0; i < p.size; i++ {
		wg.Add(1)
		workerID := fmt.Sprintf("worker-%d", i+1)
		consumerName := fmt.Sprintf("%s-%s", p.consumerGroup, workerID)

		go func(id string, name string) {
			defer wg.Done()

			w := New(id, p.db, p.registry, p.requeuer)
			consumer := queue.NewConsumer(
				p.redisURL,
				p.streamName,
				p.consumerGroup,
				name,
				w,
				p.db,
			)

			if err := consumer.EnsureConsumerGroup(ctx); err != nil {
				log.Printf("[Pool] Worker %s failed to setup consumer group: %v", id, err)
				return
			}

			consumer.Start(ctx)
		}(workerID, consumerName)
	}

	wg.Wait()
	log.Println("[Pool] All workers stopped")
}

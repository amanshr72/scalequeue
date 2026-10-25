package worker

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"sync"
	"time"
	"worker-service/internal/config"
	"worker-service/internal/db"
	"worker-service/internal/handlers"
	"worker-service/internal/queue"
)

func Run(stop context.Context, cfg *config.Config, store *db.Store, client redis.UniversalClient) error {
	execution, cancel := context.WithCancel(context.Background())
	defer cancel()
	drained := make(chan struct{})
	defer close(drained)
	go func() {
		select {
		case <-drained:
			return
		case <-stop.Done():
		}
		timer := time.NewTimer(cfg.ShutdownTimeout)
		defer timer.Stop()
		select {
		case <-drained:
		case <-timer.C:
			cancel()
		}
	}()
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	// Registry is immutable after construction and safe for concurrent reads.
	registry := handlers.NewRegistry()
	var wg sync.WaitGroup
	for i := 0; i < cfg.WorkerCount; i++ {
		id := fmt.Sprintf("%x-%d", nonce, i)
		w := &Worker{ID: id, Store: store, Registry: registry, Lease: cfg.Lease}
		c := &queue.Consumer{Client: client, Stream: cfg.StreamName, Group: cfg.ConsumerGroup, Name: id, Worker: w, RecoveryInterval: cfg.RecoveryInterval, RecoveryIdle: cfg.RecoveryIdle, JobTimeout: cfg.JobTimeout}
		if err := c.EnsureConsumerGroup(stop); err != nil {
			return err
		}
		wg.Add(1)
		go func() { defer wg.Done(); c.Start(stop, execution) }()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		publish := queue.Publish(client, cfg.StreamName)
		for stop.Err() == nil {
			ctx, cancel := context.WithTimeout(stop, 5*time.Second)
			more, err := store.Dispatch(ctx, publish)
			cancel()
			if err != nil {
				slog.Error("outbox dispatch failed", "error", err)
			}
			if err != nil || !more {
				select {
				case <-stop.Done():
					return
				case <-time.After(250 * time.Millisecond):
				}
			}
		}
	}()
	wg.Wait()
	return nil
}

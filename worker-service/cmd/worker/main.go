package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"worker-service/internal/config"
	"worker-service/internal/db"
	"worker-service/internal/handlers"
	"worker-service/internal/worker"
)

func main() {
	log.Println("[Main] ScaleQueue Worker starting...")

	// Load config
	cfg := config.Load()

	// Connect to MySQL
	database := db.NewConnection(cfg.MySQLURL)
	defer database.Close()

	// Build handler registry
	registry := handlers.NewRegistry()

	// Create worker pool
	pool := worker.NewPool(
		cfg.WorkerCount,
		database,
		registry,
		cfg.RedisURL,
		cfg.StreamName,
		cfg.ConsumerGroup,
	)

	// Graceful shutdown — listen for SIGINT (Ctrl+C) and SIGTERM (Railway stop)
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("[Main] Received signal %s — shutting down gracefully...", sig)
		cancel() // this triggers all consumers to stop via ctx.Done()
	}()

	// Start the pool — blocks until all workers stop
	pool.Start(ctx)

	log.Println("[Main] Worker service stopped cleanly")
}

package main

import (
	"context"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"worker-service/internal/config"
	"worker-service/internal/db"
	"worker-service/internal/metrics"
	"worker-service/internal/worker"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	stop, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	startup, closeStartup := context.WithTimeout(stop, 10*time.Second)
	defer closeStartup()
	database, err := db.Open(startup, cfg.MySQLURL)
	if err != nil {
		return err
	}
	defer database.Close()
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return err
	}
	client := redis.NewClient(opt)
	defer client.Close()
	if err = client.Ping(startup).Err(); err != nil {
		return err
	}
	store := &db.Store{DB: database, Lease: cfg.Lease}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok\n")) })
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, c := context.WithTimeout(r.Context(), 2*time.Second)
		defer c()
		if store.Ready(ctx) != nil || client.Ping(ctx).Err() != nil {
			http.Error(w, "not ready", 503)
			return
		}
		w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/metrics", metrics.Handler(database))
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if e := server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			slog.Error("health server failed", "error", e)
			cancel()
		}
	}()
	err = worker.Run(stop, cfg, store, client)
	ctx, c := context.WithTimeout(context.Background(), 3*time.Second)
	defer c()
	_ = server.Shutdown(ctx)
	return err
}

package db_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"os"
	"sync"
	"testing"
	"time"
	"worker-service/internal/db"
	"worker-service/internal/handlers"
	"worker-service/internal/models"
	"worker-service/internal/queue"
	"worker-service/internal/worker"
)

func database(t *testing.T) (*db.Store, context.Context) {
	t.Helper()
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN not set")
	}
	ctx := context.Background()
	d, e := db.Open(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { d.Close() })
	return &db.Store{DB: d, Lease: time.Second}, ctx
}
func job(t *testing.T, d *sql.DB) string {
	t.Helper()
	id := fmt.Sprintf("test-%d", time.Now().UnixNano())
	_, e := d.Exec(`INSERT INTO jobs(id,type,payload,max_retries) VALUES(?,'send_email','{"to":"demo@example.test"}',2)`, id)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { d.Exec("DELETE FROM jobs WHERE id=?", id) })
	return id
}
func TestIntegrationConcurrentClaimAndLeaseFencing(t *testing.T) {
	s, ctx := database(t)
	id := job(t, s.DB)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var winner *models.Job
	wins := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			j, e := s.Claim(ctx, id, 1, fmt.Sprint(i))
			if e != nil && !errors.Is(e, db.ErrBusy) {
				t.Error(e)
			}
			if j != nil {
				mu.Lock()
				wins++
				winner = j
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("claims=%d", wins)
	}
	_, e := s.DB.Exec("UPDATE jobs SET lease_until=TIMESTAMPADD(SECOND,-1,NOW(6)) WHERE id=?", id)
	if e != nil {
		t.Fatal(e)
	}
	replacement, e := s.Claim(ctx, id, 1, "replacement")
	if e != nil || replacement == nil {
		t.Fatalf("reclaim: %v", e)
	}
	if e = s.Finish(ctx, winner, models.StatusCompleted, "stale", 0); !errors.Is(e, db.ErrLostLease) {
		t.Fatalf("stale write: %v", e)
	}
	if e = s.Renew(ctx, winner); !errors.Is(e, db.ErrLostLease) {
		t.Fatalf("stale heartbeat: %v", e)
	}
	if e = s.Finish(ctx, replacement, models.StatusCompleted, "ok", 0); e != nil {
		t.Fatal(e)
	}
	j, e := s.Claim(ctx, id, 1, "duplicate")
	if e != nil || j != nil {
		t.Fatalf("terminal duplicate: %v %v", j, e)
	}
}
func TestIntegrationRetryAndReplayGeneration(t *testing.T) {
	s, ctx := database(t)
	id := job(t, s.DB)
	j, e := s.Claim(ctx, id, 1, "a")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Finish(ctx, j, models.StatusPending, "retry", time.Hour); e != nil {
		t.Fatal(e)
	}
	var outbox int
	if e = s.DB.QueryRow("SELECT COUNT(*) FROM job_outbox WHERE job_id=?", id).Scan(&outbox); e != nil || outbox != 1 {
		t.Fatalf("retry not durable %d %v", outbox, e)
	}
	if j, e = s.Claim(ctx, id, 1, "early"); e != nil || j != nil {
		t.Fatal("retry executed before due time")
	}
	s.DB.Exec("UPDATE jobs SET status='PENDING',generation=2,attempt_count=0,next_attempt_at=NOW(6) WHERE id=?", id)
	if j, e = s.Claim(ctx, id, 1, "old-generation"); e != nil || j != nil {
		t.Fatal("old message executed replay")
	}
	if j, e = s.Claim(ctx, id, 2, "new-generation"); e != nil || j == nil {
		t.Fatalf("new generation failed %v", e)
	}
}
func TestIntegrationExhaustedCrashBecomesDead(t *testing.T) {
	s, ctx := database(t)
	id := job(t, s.DB)
	s.DB.Exec("UPDATE jobs SET attempt_count=3,status='PROCESSING',lease_until=TIMESTAMPADD(SECOND,-1,NOW(6)) WHERE id=?", id)
	j, e := s.Claim(ctx, id, 1, "recovery")
	if e != nil || j != nil {
		t.Fatal(e)
	}
	var status string
	s.DB.QueryRow("SELECT status FROM jobs WHERE id=?", id).Scan(&status)
	if status != "DEAD" {
		t.Fatal(status)
	}
}
func TestIntegrationAmbiguousPublishIsSafe(t *testing.T) {
	s, ctx := database(t)
	url := os.Getenv("REDIS_TEST_URL")
	if url == "" {
		t.Skip("REDIS_TEST_URL not set")
	}
	opt, e := redis.ParseURL(url)
	if e != nil {
		t.Fatal(e)
	}
	client := redis.NewClient(opt)
	defer client.Close()
	stream := fmt.Sprintf("sq-test-%d", time.Now().UnixNano())
	defer client.Del(ctx, stream)
	id := job(t, s.DB)
	_, e = s.DB.Exec("INSERT INTO job_outbox(job_id,generation,attempt,available_at) VALUES(?,1,1,NOW(6))", id)
	if e != nil {
		t.Fatal(e)
	}
	publish := queue.Publish(client, stream)
	_, e = s.Dispatch(ctx, func(c context.Context, id string, g int) error {
		if e := publish(c, id, g); e != nil {
			return e
		}
		return errors.New("crash after XADD before SQL commit")
	})
	if e == nil {
		t.Fatal("injected failure absent")
	}
	more, e := s.Dispatch(ctx, publish)
	if e != nil || !more {
		t.Fatalf("outbox did not recover %v", e)
	}
	messages, e := client.XRange(ctx, stream, "-", "+").Result()
	if e != nil || len(messages) != 2 {
		t.Fatalf("expected duplicate deliveries: %d %v", len(messages), e)
	}
	w := worker.Worker{ID: "demo", Store: s, Registry: handlers.NewRegistry(), Lease: time.Second}
	for range messages {
		if e = w.ProcessJob(ctx, id, 1); e != nil {
			t.Fatal(e)
		}
	}
	var attempts int
	s.DB.QueryRow("SELECT attempt_count FROM jobs WHERE id=?", id).Scan(&attempts)
	if attempts != 1 {
		t.Fatalf("duplicate executed %d times", attempts)
	}
}

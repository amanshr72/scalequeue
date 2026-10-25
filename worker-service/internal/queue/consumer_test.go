package queue

import (
	"context"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"os"
	"testing"
	"time"
)

type testProcessor struct {
	calls int
	err   error
}

func (p *testProcessor) ProcessJob(context.Context, string, int) error { p.calls++; return p.err }
func clientFor(t *testing.T) (*Consumer, *testProcessor, context.Context) {
	t.Helper()
	url := os.Getenv("REDIS_TEST_URL")
	if url == "" {
		t.Skip("REDIS_TEST_URL not set")
	}
	opt, e := redis.ParseURL(url)
	if e != nil {
		t.Fatal(e)
	}
	client := redis.NewClient(opt)
	ctx := context.Background()
	stream := fmt.Sprintf("queue-test-%d", time.Now().UnixNano())
	p := &testProcessor{}
	c := &Consumer{Client: client, Stream: stream, Group: "test", Name: "new", Worker: p, RecoveryIdle: time.Millisecond, JobTimeout: time.Second, cursor: "0-0"}
	if e = c.EnsureConsumerGroup(ctx); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { client.Del(ctx, stream); client.Close() })
	return c, p, ctx
}
func TestIntegrationRecoveryBeyondFirstHundred(t *testing.T) {
	c, p, ctx := clientFor(t)
	for i := 0; i < 150; i++ {
		c.Client.XAdd(ctx, &redis.XAddArgs{Stream: c.Stream, Values: map[string]interface{}{"job_id": fmt.Sprint(i), "generation": 1}})
	}
	_, e := c.Client.XReadGroup(ctx, &redis.XReadGroupArgs{Group: c.Group, Consumer: "crashed", Streams: []string{c.Stream, ">"}, Count: 150}).Result()
	if e != nil {
		t.Fatal(e)
	}
	time.Sleep(5 * time.Millisecond)
	for i := 0; i < 10 && p.calls < 150; i++ {
		c.recover(ctx, ctx)
	}
	if p.calls != 150 {
		t.Fatalf("only recovered %d", p.calls)
	}
	pending, e := c.Client.XPending(ctx, c.Stream, c.Group).Result()
	if e != nil || pending.Count != 0 {
		t.Fatalf("pending: %v %v", pending, e)
	}
}
func TestIntegrationFailedAndMalformedRemainPending(t *testing.T) {
	c, p, ctx := clientFor(t)
	p.err = errors.New("database failed")
	for _, values := range []map[string]interface{}{{"job_id": "job", "generation": 1}, {"bad": "data"}} {
		c.Client.XAdd(ctx, &redis.XAddArgs{Stream: c.Stream, Values: values})
	}
	streams, e := c.Client.XReadGroup(ctx, &redis.XReadGroupArgs{Group: c.Group, Consumer: "worker", Streams: []string{c.Stream, ">"}, Count: 2}).Result()
	if e != nil {
		t.Fatal(e)
	}
	for _, m := range streams[0].Messages {
		c.handle(ctx, m)
	}
	pending, e := c.Client.XPending(ctx, c.Stream, c.Group).Result()
	if e != nil || pending.Count != 2 {
		t.Fatalf("failed deliveries ACKed: %v %v", pending, e)
	}
}

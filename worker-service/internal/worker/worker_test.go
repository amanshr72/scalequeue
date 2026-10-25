package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"worker-service/internal/handlers"
	"worker-service/internal/models"
)

type fakeStore struct {
	job                           *models.Job
	claimErr, finishErr, renewErr error
	finished                      atomic.Int32
	status                        models.JobStatus
}

func (s *fakeStore) Claim(context.Context, string, int, string) (*models.Job, error) {
	return s.job, s.claimErr
}
func (s *fakeStore) Renew(context.Context, *models.Job) error { return s.renewErr }
func (s *fakeStore) Finish(_ context.Context, _ *models.Job, status models.JobStatus, _ string, _ time.Duration) error {
	s.finished.Add(1)
	s.status = status
	return s.finishErr
}

type handlerFunc func(context.Context, *models.Job) error

func (h handlerFunc) Handle(c context.Context, j *models.Job) error { return h(c, j) }
func TestOutcomeErrorsAreNotAcknowledgable(t *testing.T) {
	failure := errors.New("db unavailable")
	for _, stage := range []string{"claim", "finish", "renew"} {
		t.Run(stage, func(t *testing.T) {
			s := &fakeStore{job: &models.Job{ID: "job", Type: "test", AttemptCount: 1, MaxRetries: 3}}
			h := handlerFunc(func(context.Context, *models.Job) error { return nil })
			switch stage {
			case "claim":
				s.claimErr = failure
			case "finish":
				s.finishErr = failure
			case "renew":
				s.renewErr = failure
				h = func(ctx context.Context, _ *models.Job) error { <-ctx.Done(); return ctx.Err() }
			}
			reg := handlers.NewRegistry()
			reg.Register("test", h)
			w := &Worker{ID: "test", Store: s, Registry: reg, Lease: 30 * time.Millisecond}
			if err := w.ProcessJob(context.Background(), "job", 1); !errors.Is(err, failure) {
				t.Fatalf("expected failure, got %v", err)
			}
			if stage != "finish" && s.finished.Load() != 0 {
				t.Fatal("persisted result without ownership")
			}
		})
	}
}
func TestOutcomeClassification(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		attempt int
		want    models.JobStatus
	}{{"success", nil, 1, models.StatusCompleted}, {"retry", errors.New("transient"), 1, models.StatusPending}, {"exhausted", errors.New("transient"), 4, models.StatusDead}, {"permanent", handlers.Permanent(errors.New("bad payload")), 1, models.StatusDead}} {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeStore{job: &models.Job{ID: "job", Type: "test", AttemptCount: tc.attempt, MaxRetries: 3}}
			reg := handlers.NewRegistry()
			reg.Register("test", handlerFunc(func(context.Context, *models.Job) error { return tc.err }))
			w := Worker{ID: "w", Store: s, Registry: reg, Lease: time.Second}
			if err := w.ProcessJob(context.Background(), "job", 1); err != nil {
				t.Fatal(err)
			}
			if s.status != tc.want {
				t.Fatalf("%s != %s", s.status, tc.want)
			}
		})
	}
}
func TestPanicLeavesDeliveryRecoverable(t *testing.T) {
	s := &fakeStore{job: &models.Job{Type: "panic"}}
	reg := handlers.NewRegistry()
	reg.Register("panic", handlerFunc(func(context.Context, *models.Job) error { panic("boom") }))
	w := Worker{Store: s, Registry: reg, Lease: time.Second}
	if w.ProcessJob(context.Background(), "job", 1) == nil {
		t.Fatal("panic acknowledged")
	}
	if s.finished.Load() != 0 {
		t.Fatal("panic persisted as success")
	}
}

// Regression: completing a handler while a heartbeat query is in flight must not
// cancel that query and turn a successful job into a stranded delivery.
type delayedRenewStore struct {
	fakeStore
	started chan struct{}
	once    sync.Once
}

func (s *delayedRenewStore) Renew(ctx context.Context, j *models.Job) error {
	s.once.Do(func() { close(s.started) })
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(15 * time.Millisecond):
		return nil
	}
}
func TestCompletionWaitsForInflightHeartbeat(t *testing.T) {
	s := &delayedRenewStore{fakeStore: fakeStore{job: &models.Job{Type: "wait", AttemptCount: 1, MaxRetries: 3}}, started: make(chan struct{})}
	reg := handlers.NewRegistry()
	reg.Register("wait", handlerFunc(func(context.Context, *models.Job) error { <-s.started; return nil }))
	w := Worker{Store: s, Registry: reg, Lease: 30 * time.Millisecond}
	if e := w.ProcessJob(context.Background(), "job", 1); e != nil {
		t.Fatal(e)
	}
	if s.finished.Load() != 1 || s.status != models.StatusCompleted {
		t.Fatal("completion lost")
	}
}

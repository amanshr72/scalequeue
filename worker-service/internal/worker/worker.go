package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"worker-service/internal/handlers"
	"worker-service/internal/metrics"
	"worker-service/internal/models"
	"worker-service/internal/retry"
)

type JobStore interface {
	Claim(context.Context, string, int, string) (*models.Job, error)
	Renew(context.Context, *models.Job) error
	Finish(context.Context, *models.Job, models.JobStatus, string, time.Duration) error
}
type Worker struct {
	ID       string
	Store    JobStore
	Registry *handlers.Registry
	Lease    time.Duration
}

func (w *Worker) ProcessJob(ctx context.Context, id string, generation int) (err error) {
	j, err := w.Store.Claim(ctx, id, generation, w.ID)
	if err != nil || j == nil {
		return err
	}
	metrics.InFlight.Add(1)
	defer metrics.InFlight.Add(-1)
	metrics.Attempts.Add(1)
	started := time.Now()
	defer func() {
		metrics.ProcessingNanos.Add(uint64(time.Since(started)))
		if p := recover(); p != nil {
			err = fmt.Errorf("handler panic: %v", p)
		}
	}()
	task, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	stopRenew := make(chan struct{})
	renewed := make(chan error, 1)
	go func() {
		defer close(done)
		tick := time.NewTicker(w.Lease / 3)
		defer tick.Stop()
		for {
			select {
			case <-task.Done():
				return
			case <-stopRenew:
				return
			case <-tick.C:
				if e := w.Store.Renew(task, j); e != nil {
					renewed <- e
					cancel()
					return
				}
			}
		}
	}()
	handler, e := w.Registry.Get(j.Type)
	if e == nil {
		e = handler.Handle(task, j)
	} else {
		e = handlers.Permanent(e)
	}
	close(stopRenew)
	<-done
	cancel()
	select {
	case e2 := <-renewed:
		return e2
	default:
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	status := models.StatusCompleted
	message := "Completed successfully"
	delay := time.Duration(0)
	if e != nil {
		status = models.StatusPending
		message = e.Error()
		delay = retry.ExponentialDelay(j.RetryCount)
		var permanent *handlers.PermanentError
		if errors.As(e, &permanent) || j.AttemptCount >= j.MaxRetries+1 {
			status = models.StatusDead
		}
	}
	// Use the caller context, not the canceled heartbeat/handler context.
	if err = w.Store.Finish(ctx, j, status, message, delay); err != nil {
		return err
	}
	switch status {
	case models.StatusCompleted:
		metrics.Completed.Add(1)
	case models.StatusDead:
		metrics.Dead.Add(1)
	case models.StatusPending:
		metrics.Retries.Add(1)
	}
	slog.Info("job outcome persisted", "job_id", id, "generation", generation, "attempt", j.AttemptCount, "status", status, "worker", w.ID)
	return nil
}

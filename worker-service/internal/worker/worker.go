package worker

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"worker-service/internal/db"
	"worker-service/internal/handlers"
	"worker-service/internal/models"
	"worker-service/internal/queue"
	"worker-service/internal/retry"
)

type Worker struct {
	id       string
	db       *sql.DB
	registry *handlers.Registry
	requeuer *queue.Requeuer
}

func New(id string, database *sql.DB, registry *handlers.Registry, requeuer *queue.Requeuer) *Worker {
	return &Worker{
		id:       id,
		db:       database,
		registry: registry,
		requeuer: requeuer,
	}
}

func (w *Worker) ProcessJob(ctx context.Context, jobID string, streamMsgID string) error {
	log.Printf("[Worker %s] Starting job %s", w.id, jobID)

	job, err := db.GetJobByID(w.db, jobID)
	if err != nil {
		return fmt.Errorf("failed to fetch job %s: %w", jobID, err)
	}
	if job == nil {
		log.Printf("[Worker %s] Job %s not found in MySQL, skipping", w.id, jobID)
		return nil
	}

	// Idempotency guard
	if job.Status == models.StatusCompleted || job.Status == models.StatusDead {
		log.Printf("[Worker %s] Job %s already terminal (%s), skipping", w.id, jobID, job.Status)
		return nil
	}

	// Mark PROCESSING
	if err := db.UpdateJobStatus(w.db, jobID, models.StatusProcessing, nil); err != nil {
		return fmt.Errorf("failed to mark PROCESSING: %w", err)
	}
	db.WriteAuditLog(w.db, jobID,
		string(job.Status), string(models.StatusProcessing),
		"Worker picked up job", w.id)

	// Find handler
	handler, err := w.registry.Get(job.Type)
	if err != nil {
		errMsg := err.Error()
		db.UpdateJobStatus(w.db, jobID, models.StatusDead, &errMsg)
		db.WriteAuditLog(w.db, jobID,
			string(models.StatusProcessing), string(models.StatusDead),
			errMsg, w.id)
		log.Printf("[Worker %s] No handler for type '%s' — marking DEAD", w.id, job.Type)
		return nil
	}

	// Execute handler
	handlerErr := handler.Handle(job)

	// Success path
	if handlerErr == nil {
		db.UpdateJobStatus(w.db, jobID, models.StatusCompleted, nil)
		db.WriteAuditLog(w.db, jobID,
			string(models.StatusProcessing), string(models.StatusCompleted),
			"Completed successfully", w.id)
		log.Printf("[Worker %s] Job %s COMPLETED ✓", w.id, jobID)
		return nil
	}

	// Failure path
	errMsg := handlerErr.Error()
	log.Printf("[Worker %s] Job %s FAILED: %s (attempt %d/%d)",
		w.id, jobID, errMsg, job.RetryCount+1, job.MaxRetries)

	if job.RetryCount >= job.MaxRetries {
		// Dead letter
		db.UpdateJobStatus(w.db, jobID, models.StatusDead, &errMsg)
		db.WriteAuditLog(w.db, jobID,
			string(models.StatusProcessing), string(models.StatusDead),
			fmt.Sprintf("Max retries (%d) exhausted: %s", job.MaxRetries, errMsg), w.id)
		log.Printf("[Worker %s] Job %s → DEAD after %d retries", w.id, jobID, job.MaxRetries)
		return nil
	}

	// Schedule retry with exponential backoff
	delay := retry.ExponentialDelay(job.RetryCount)
	db.IncrementRetryCount(w.db, jobID, errMsg)
	db.WriteAuditLog(w.db, jobID,
		string(models.StatusProcessing), string(models.StatusPending),
		fmt.Sprintf("Retry %d/%d in %s: %s", job.RetryCount+1, job.MaxRetries, delay, errMsg),
		w.id)

	log.Printf("[Worker %s] Job %s retrying in %s...", w.id, jobID, delay)
	time.Sleep(delay)

	// Re-enqueue directly (no RETRY: signal needed anymore)
	if err := w.requeuer.Enqueue(ctx, jobID, job.Type); err != nil {
		log.Printf("[Worker %s] Failed to re-enqueue job %s: %v", w.id, jobID, err)
	}

	return nil
}

package handlers

import (
	"fmt"
	"log"
	"worker-service/internal/models"
)

// FailingHandler always errors — used to test retry + dead letter flow
type FailingHandler struct{}

func (h *FailingHandler) Handle(job *models.Job) error {
	log.Printf("[FailingHandler] Job %s deliberately failing (retry_count=%d)",
		job.ID, job.RetryCount)
	return fmt.Errorf("simulated failure for testing retry logic")
}

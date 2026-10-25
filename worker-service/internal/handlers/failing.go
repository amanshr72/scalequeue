package handlers

import (
	"context"
	"fmt"
	"worker-service/internal/models"
)

type FailingHandler struct{}

func (h *FailingHandler) Handle(ctx context.Context, j *models.Job) error {
	return fmt.Errorf("simulated transient failure")
}

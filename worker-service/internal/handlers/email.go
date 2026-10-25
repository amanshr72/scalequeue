package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"worker-service/internal/models"
)

type EmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}
type EmailHandler struct{}

func (h *EmailHandler) Handle(ctx context.Context, j *models.Job) error {
	var p EmailPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return Permanent(err)
	}
	if p.To == "" {
		return Permanent(fmt.Errorf("email payload missing to"))
	}
	return wait(ctx, 500*time.Millisecond)
}
func wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

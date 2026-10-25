package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"worker-service/internal/models"
)

type NotificationPayload struct {
	UserID  string `json:"user_id"`
	Message string `json:"message"`
	Channel string `json:"channel"`
}
type NotificationHandler struct{}

func (h *NotificationHandler) Handle(ctx context.Context, j *models.Job) error {
	var p NotificationPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return Permanent(err)
	}
	if p.UserID == "" {
		return Permanent(fmt.Errorf("notification payload missing user_id"))
	}
	return wait(ctx, 300*time.Millisecond)
}

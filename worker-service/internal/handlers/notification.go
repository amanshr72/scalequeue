package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"worker-service/internal/models"
)

type NotificationPayload struct {
	UserID  string `json:"user_id"`
	Message string `json:"message"`
	Channel string `json:"channel"` // "push", "sms", "in_app"
}

type NotificationHandler struct{}

func (h *NotificationHandler) Handle(job *models.Job) error {
	var payload NotificationPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("invalid notification payload: %w", err)
	}

	log.Printf("[NotificationHandler] Sending %s notification to user %s",
		payload.Channel, payload.UserID)
	time.Sleep(300 * time.Millisecond)

	log.Printf("[NotificationHandler] Notification delivered to user %s", payload.UserID)
	return nil
}

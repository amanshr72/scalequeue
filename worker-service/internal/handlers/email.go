package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"worker-service/internal/models"
)

type EmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type EmailHandler struct{}

func (h *EmailHandler) Handle(job *models.Job) error {
	var payload EmailPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		// Malformed payload — returning error will cause retry
		// but retrying won't fix a bad payload, so we return
		// a special marker. We'll handle this in Phase 2.
		return fmt.Errorf("invalid email payload: %w", err)
	}

	if payload.To == "" {
		return fmt.Errorf("email payload missing 'to' field")
	}

	// Simulate real email sending work
	// In Phase 2, replace with actual SMTP or SendGrid call
	log.Printf("[EmailHandler] Sending email to %s | Subject: %s", payload.To, payload.Subject)
	time.Sleep(500 * time.Millisecond) // simulate network call

	log.Printf("[EmailHandler] Email sent successfully to %s", payload.To)
	return nil
}

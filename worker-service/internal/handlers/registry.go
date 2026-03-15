package handlers

import (
	"fmt"

	"worker-service/internal/models"
)

// Handler is the interface every job type must implement
type Handler interface {
	Handle(job *models.Job) error
}

// Registry maps job types to their handlers
type Registry struct {
	handlers map[string]Handler
}

func NewRegistry() *Registry {
	r := &Registry{
		handlers: make(map[string]Handler),
	}

	// Register all handlers here
	r.Register("send_email", &EmailHandler{})               // Email
	r.Register("send_notification", &NotificationHandler{}) // Notification
	r.Register("fail_job", &FailingHandler{})               // DLQ

	return r
}

func (r *Registry) Register(jobType string, handler Handler) {
	r.handlers[jobType] = handler
}

func (r *Registry) Get(jobType string) (Handler, error) {
	h, exists := r.handlers[jobType]
	if !exists {
		return nil, fmt.Errorf("no handler registered for job type: %s", jobType)
	}
	return h, nil
}

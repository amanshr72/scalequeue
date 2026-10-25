package handlers

import (
	"context"
	"fmt"
	"worker-service/internal/models"
)

type Handler interface {
	Handle(context.Context, *models.Job) error
}
type PermanentError struct{ Err error }

func (e *PermanentError) Error() string { return e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }
func Permanent(e error) error           { return &PermanentError{e} }

type Registry struct{ handlers map[string]Handler }

func NewRegistry() *Registry {
	r := &Registry{handlers: map[string]Handler{}}
	r.Register("send_email", &EmailHandler{})
	r.Register("send_notification", &NotificationHandler{})
	r.Register("fail_job", &FailingHandler{})
	return r
}
func (r *Registry) Register(t string, h Handler) { r.handlers[t] = h }
func (r *Registry) Get(t string) (Handler, error) {
	h, ok := r.handlers[t]
	if !ok {
		return nil, fmt.Errorf("no handler registered for %s", t)
	}
	return h, nil
}

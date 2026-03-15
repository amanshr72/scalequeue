package models

import "time"

type JobStatus string

const (
	StatusPending    JobStatus = "PENDING"
	StatusProcessing JobStatus = "PROCESSING"
	StatusCompleted  JobStatus = "COMPLETED"
	StatusFailed     JobStatus = "FAILED"
	StatusDead       JobStatus = "DEAD"
)

type Job struct {
	ID          string
	Type        string
	Payload     []byte // raw JSON — handlers will unmarshal what they need
	Status      JobStatus
	RetryCount  int
	MaxRetries  int
	Priority    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ProcessedAt *time.Time
	ErrorMsg    *string
}

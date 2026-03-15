package db

import (
	"database/sql"
	"log"
	"time"
	"worker-service/internal/models"

	_ "github.com/go-sql-driver/mysql"
)

func NewConnection(dsn string) *sql.DB {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("[DB] Failed to open MySQL connection: %v", err)
	}

	// Connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection is actually alive
	if err := db.Ping(); err != nil {
		log.Fatalf("[DB] Failed to ping MySQL: %v", err)
	}

	log.Println("[DB] MySQL connected successfully")
	return db
}

// GetJobByID fetches a full job record from MySQL
func GetJobByID(db *sql.DB, jobID string) (*models.Job, error) {
	row := db.QueryRow(
		`SELECT id, type, payload, status, retry_count, max_retries,
                priority, created_at, updated_at, processed_at, error_msg
         FROM jobs WHERE id = ?`,
		jobID,
	)

	var job models.Job
	err := row.Scan(
		&job.ID,
		&job.Type,
		&job.Payload,
		&job.Status,
		&job.RetryCount,
		&job.MaxRetries,
		&job.Priority,
		&job.CreatedAt,
		&job.UpdatedAt,
		&job.ProcessedAt,
		&job.ErrorMsg,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// UpdateJobStatus updates status and optionally sets error message
func UpdateJobStatus(db *sql.DB, jobID string, status models.JobStatus, errMsg *string) error {
	if status == models.StatusCompleted || status == models.StatusFailed || status == models.StatusDead {
		_, err := db.Exec(
			`UPDATE jobs 
             SET status = ?, error_msg = ?, processed_at = NOW(), updated_at = NOW()
             WHERE id = ?`,
			status, errMsg, jobID,
		)
		return err
	}

	_, err := db.Exec(
		`UPDATE jobs SET status = ?, updated_at = NOW() WHERE id = ?`,
		status, jobID,
	)
	return err
}

// IncrementRetryCount bumps retry count and resets status to PENDING
func IncrementRetryCount(db *sql.DB, jobID string, errMsg string) error {
	_, err := db.Exec(
		`UPDATE jobs 
         SET retry_count = retry_count + 1,
             status = 'PENDING',
             error_msg = ?,
             updated_at = NOW()
         WHERE id = ?`,
		errMsg, jobID,
	)
	return err
}

// WriteAuditLog records a job status transition
func WriteAuditLog(db *sql.DB, jobID, fromStatus, toStatus, message, workerID string) {
	_, err := db.Exec(
		`INSERT INTO job_audit_logs (job_id, from_status, to_status, message, worker_id)
         VALUES (?, ?, ?, ?, ?)`,
		jobID, fromStatus, toStatus, message, workerID,
	)
	if err != nil {
		// Audit log failure should never crash the worker
		log.Printf("[AuditLog] Failed to write audit log for job %s: %v", jobID, err)
	}
}

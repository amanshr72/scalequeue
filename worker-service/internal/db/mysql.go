package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"time"
	"worker-service/internal/models"
)

var ErrBusy = errors.New("job has an active lease")
var ErrLostLease = errors.New("job lease lost")

type Store struct {
	DB    *sql.DB
	Lease time.Duration
}

func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	d, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(20)
	d.SetMaxIdleConns(10)
	d.SetConnMaxLifetime(5 * time.Minute)
	if err = d.PingContext(ctx); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

// Claim serializes competing deliveries in MySQL. A lease fences later writes.
func (s *Store) Claim(ctx context.Context, id string, generation int, owner string) (*models.Job, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var j models.Job
	var active, early bool
	err = tx.QueryRowContext(ctx, `SELECT id,type,payload,status,retry_count,max_retries,
 priority,created_at,updated_at,processed_at,error_msg,generation,attempt_count,
 COALESCE(lease_until>NOW(6),FALSE),next_attempt_at>NOW(6)
 FROM jobs WHERE id=? FOR UPDATE`, id).Scan(&j.ID, &j.Type, &j.Payload, &j.Status, &j.RetryCount, &j.MaxRetries, &j.Priority, &j.CreatedAt, &j.UpdatedAt, &j.ProcessedAt, &j.ErrorMsg, &j.Generation, &j.AttemptCount, &active, &early)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if j.Generation != generation || j.Status == models.StatusCompleted || j.Status == models.StatusDead {
		return nil, nil
	}
	if j.Status == models.StatusProcessing && active {
		return nil, ErrBusy
	}
	if early {
		return nil, nil
	} // the durable retry outbox owns the next delivery
	if j.AttemptCount >= j.MaxRetries+1 {
		_, err = tx.ExecContext(ctx, `UPDATE jobs SET status='DEAD',error_msg='Attempt budget exhausted after interrupted execution',lease_owner=NULL,lease_until=NULL,processed_at=NOW(6) WHERE id=?`, id)
		if err != nil {
			return nil, err
		}
		if err = audit(ctx, tx, id, string(j.Status), "DEAD", "Attempt budget exhausted after interrupted execution", owner); err != nil {
			return nil, err
		}
		return nil, tx.Commit()
	}
	_, err = tx.ExecContext(ctx, `UPDATE jobs SET status='PROCESSING',attempt_count=attempt_count+1,
 retry_count=GREATEST(attempt_count-1,0),lease_owner=?,lease_until=TIMESTAMPADD(MICROSECOND,?,NOW(6)),updated_at=NOW(6) WHERE id=?`, owner, s.Lease.Microseconds(), id)
	if err != nil {
		return nil, err
	}
	if err = audit(ctx, tx, id, string(j.Status), "PROCESSING", "Attempt claimed", owner); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	j.AttemptCount++
	j.RetryCount = j.AttemptCount - 1
	j.Status = models.StatusProcessing
	j.LeaseOwner = owner
	return &j, nil
}
func (s *Store) Renew(ctx context.Context, j *models.Job) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE jobs SET lease_until=TIMESTAMPADD(MICROSECOND,?,NOW(6)) WHERE id=? AND generation=? AND status='PROCESSING' AND lease_owner=? AND lease_until>NOW(6)`, s.Lease.Microseconds(), j.ID, j.Generation, j.LeaseOwner)
	return owned(result, err)
}
func owned(r sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrLostLease
	}
	return nil
}
func audit(ctx context.Context, tx *sql.Tx, id, from, to, message, owner string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO job_audit_logs(job_id,from_status,to_status,message,worker_id) VALUES(?,?,?,?,?)`, id, from, to, message, owner)
	return err
}

// Finish atomically persists the outcome, audit event, and any scheduled retry.
func (s *Store) Finish(ctx context.Context, j *models.Job, status models.JobStatus, message string, delay time.Duration) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE jobs SET status=?,error_msg=NULLIF(?,''),
 next_attempt_at=TIMESTAMPADD(MICROSECOND,?,NOW(6)),lease_owner=NULL,lease_until=NULL,
 processed_at=IF(?='PENDING',NULL,NOW(6)),updated_at=NOW(6)
 WHERE id=? AND generation=? AND status='PROCESSING' AND lease_owner=? AND lease_until>NOW(6)`, status, message, delay.Microseconds(), status, j.ID, j.Generation, j.LeaseOwner)
	if err = owned(res, err); err != nil {
		return err
	}
	if err = audit(ctx, tx, j.ID, "PROCESSING", string(status), message, j.LeaseOwner); err != nil {
		return err
	}
	if status == models.StatusPending {
		_, err = tx.ExecContext(ctx, `INSERT INTO job_outbox(job_id,generation,attempt,available_at)
 SELECT id,generation,attempt_count+1,next_attempt_at FROM jobs WHERE id=?`, j.ID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Dispatch keeps the outbox row locked through publish. Crash after XADD and
// before commit duplicates delivery; deleting/marking before XADD could lose it.
func (s *Store) Dispatch(ctx context.Context, publish func(context.Context, string, int) error) (bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var id int64
	var job string
	var gen int
	err = tx.QueryRowContext(ctx, `SELECT id,job_id,generation FROM job_outbox
 WHERE available_at<=NOW(6) AND (published_at IS NULL OR published_at<TIMESTAMPADD(SECOND,-30,NOW(6)))
 ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&id, &job, &gen)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// Republish pending and expired work periodically, also healing a lost Redis stream.
	var status string
	var current int
	err = tx.QueryRowContext(ctx, "SELECT status,generation FROM jobs WHERE id=?", job).Scan(&status, &current)
	if err != nil {
		return false, err
	}
	if current != gen || status == "COMPLETED" || status == "DEAD" {
		_, err = tx.ExecContext(ctx, "DELETE FROM job_outbox WHERE id=?", id)
	} else {
		if err = publish(ctx, job, gen); err != nil {
			return false, err
		}
		_, err = tx.ExecContext(ctx, "UPDATE job_outbox SET published_at=NOW(6) WHERE id=?", id)
	}
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}
func (s *Store) Ready(ctx context.Context) error {
	if err := s.DB.PingContext(ctx); err != nil {
		return fmt.Errorf("mysql: %w", err)
	}
	return nil
}

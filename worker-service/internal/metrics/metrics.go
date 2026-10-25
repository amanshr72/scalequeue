package metrics

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

var InFlight atomic.Int64
var Attempts, Completed, Dead, Retries, Errors, ProcessingNanos atomic.Uint64

func Handler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		rows, err := db.QueryContext(ctx, "SELECT status,COUNT(*),COALESCE(MAX(TIMESTAMPDIFF(SECOND,created_at,NOW())),0) FROM jobs GROUP BY status")
		if err != nil {
			http.Error(w, "database unavailable", 503)
			return
		}
		defer rows.Close()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		for rows.Next() {
			var status string
			var n, age int64
			if err := rows.Scan(&status, &n, &age); err != nil {
				http.Error(w, "metrics query failed", 503)
				return
			}
			fmt.Fprintf(w, "scalequeue_jobs{status=%q} %d\nscalequeue_oldest_job_seconds{status=%q} %d\n", status, n, status, age)
		}
		fmt.Fprintf(w, "scalequeue_inflight %d\nscalequeue_attempts_total %d\nscalequeue_completed_total %d\nscalequeue_dead_total %d\nscalequeue_retries_total %d\nscalequeue_delivery_errors_total %d\nscalequeue_processing_seconds_sum %g\n", InFlight.Load(), Attempts.Load(), Completed.Load(), Dead.Load(), Retries.Load(), Errors.Load(), float64(ProcessingNanos.Load())/1e9)
	}
}

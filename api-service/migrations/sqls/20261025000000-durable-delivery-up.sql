ALTER TABLE jobs
  ADD COLUMN request_key CHAR(64) NULL,
  ADD COLUMN request_hash CHAR(64) NULL,
  ADD UNIQUE KEY idx_request_key (request_key),
  ADD COLUMN generation INT NOT NULL DEFAULT 1,
  ADD COLUMN attempt_count INT NOT NULL DEFAULT 0,
  ADD COLUMN next_attempt_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  ADD COLUMN lease_owner VARCHAR(100) NULL,
  ADD COLUMN lease_until DATETIME(6) NULL,
  ADD INDEX idx_recovery (status, lease_until);
CREATE TABLE job_outbox (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  job_id VARCHAR(36) NOT NULL,
  generation INT NOT NULL,
  attempt INT NOT NULL,
  available_at DATETIME(6) NOT NULL,
  published_at DATETIME(6) NULL,
  UNIQUE KEY dispatch_identity (job_id, generation, attempt),
  INDEX idx_dispatch (available_at, published_at),
  FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- Preserve consumed retry budgets during a paused-worker upgrade.
UPDATE jobs SET attempt_count=retry_count+IF(status='PROCESSING',1,0)
WHERE status IN ('PENDING','PROCESSING');
-- Existing nonterminal jobs become dispatchable during a paused-worker upgrade.
INSERT INTO job_outbox (job_id, generation, attempt, available_at)
SELECT id, generation, attempt_count+1, NOW(6) FROM jobs WHERE status IN ('PENDING','PROCESSING');
UPDATE jobs SET status='PENDING' WHERE status='PROCESSING';

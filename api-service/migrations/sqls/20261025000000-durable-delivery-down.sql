DROP TABLE job_outbox;
ALTER TABLE jobs DROP INDEX idx_request_key, DROP COLUMN request_key, DROP COLUMN request_hash, DROP INDEX idx_recovery, DROP COLUMN generation,
 DROP COLUMN attempt_count, DROP COLUMN next_attempt_at,
 DROP COLUMN lease_owner, DROP COLUMN lease_until;

CREATE TABLE jobs (
  id VARCHAR(36) NOT NULL,
  type VARCHAR(50) NOT NULL,
  payload JSON NOT NULL,
  status ENUM(
    'PENDING',
    'PROCESSING',
    'COMPLETED',
    'FAILED',
    'DEAD'
  ) NOT NULL DEFAULT 'PENDING',
  retry_count INT NOT NULL DEFAULT 0,
  max_retries INT NOT NULL DEFAULT 3,
  priority TINYINT NOT NULL DEFAULT 5,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  processed_at TIMESTAMP NULL,
  error_msg TEXT NULL,
  PRIMARY KEY (id),
  INDEX idx_status (status),
  INDEX idx_type (type),
  INDEX idx_created_at (created_at),
  INDEX idx_status_created (status, created_at) -- composite for dashboard queries
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;
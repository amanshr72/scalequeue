package retry

import "time"

// ExponentialDelay returns how long to wait before retrying
// attempt 1 → 2s, attempt 2 → 4s, attempt 3 → 8s, capped at 5 minutes
func ExponentialDelay(attempt int) time.Duration {
	base := 2 * time.Second
	delay := base * (1 << attempt) // 2^attempt * base

	maxDelay := 5 * time.Minute
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

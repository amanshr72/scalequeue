package retry

import (
	"math/rand/v2"
	"time"
)

// Equal jitter: first retry 1–2 seconds; bounded at five minutes without overflow.
func ExponentialDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	if attempt > 8 {
		attempt = 8
	}
	cap := 2 * time.Second * time.Duration(1<<attempt)
	if cap > 5*time.Minute {
		cap = 5 * time.Minute
	}
	return cap/2 + time.Duration(rand.Int64N(int64(cap/2)+1))
}

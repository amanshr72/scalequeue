package retry

import (
	"testing"
	"time"
)

func TestBackoffBounds(t *testing.T) {
	for _, attempt := range []int{-1, 0, 1, 7, 8, 1000000} {
		for i := 0; i < 100; i++ {
			d := ExponentialDelay(attempt)
			if d < time.Second || d > 5*time.Minute {
				t.Fatalf("attempt %d: %s", attempt, d)
			}
		}
	}
}

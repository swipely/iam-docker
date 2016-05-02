package utils

import (
	"time"
)

// ExponentialBackoff accepts a base time.Duration and attempt number. It
// returns the number of seconds which should be waited before attempting again.
func ExponentialBackoff(base time.Duration, attempt int) time.Duration {
	backoff := base
	for i := 1; i < attempt; i++ {
		backoff *= 2
	}
	return backoff
}

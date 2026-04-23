package engine

import "time"

func RetryDelay(attempt int) time.Duration {
	if attempt <= 1 {
		return 0
	}
	d := time.Duration(1<<(attempt-2)) * 200 * time.Millisecond
	if d > 3*time.Second {
		return 3 * time.Second
	}
	return d
}

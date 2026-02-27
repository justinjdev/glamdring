package api

import (
	"context"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

const maxRetries = 3

// retryableStatusCodes are HTTP status codes that warrant a retry.
var retryableStatusCodes = map[int]bool{
	429: true, // Rate limited
	500: true, // Server error
	529: true, // Overloaded
}

// shouldRetry returns true if the given status code is retryable.
func shouldRetry(statusCode int) bool {
	return retryableStatusCodes[statusCode]
}

// backoffDelay computes the delay for the given attempt (0-indexed).
// It uses exponential backoff with full jitter: uniform random in [0, base * 2^attempt].
// The base delay is 1 second, capped at 30 seconds before jitter.
func backoffDelay(attempt int) time.Duration {
	base := 1.0 // seconds
	cap := 30.0 // seconds
	exp := math.Min(base*math.Pow(2, float64(attempt)), cap)
	jittered := rand.Float64() * exp
	return time.Duration(jittered * float64(time.Second))
}

// retryAfterDelay parses the Retry-After header and returns the delay.
// Returns 0 if the header is missing or unparseable.
func retryAfterDelay(resp *http.Response) time.Duration {
	val := resp.Header.Get("Retry-After")
	if val == "" {
		return 0
	}
	// Try parsing as seconds (integer).
	if seconds, err := strconv.Atoi(val); err == nil {
		return time.Duration(seconds) * time.Second
	}
	// Try parsing as HTTP-date.
	if t, err := http.ParseTime(val); err == nil {
		delay := time.Until(t)
		if delay > 0 {
			return delay
		}
	}
	return 0
}

// sleepWithContext sleeps for the given duration, returning early if ctx is cancelled.
func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

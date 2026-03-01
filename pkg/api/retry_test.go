package api

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		code int
		want bool
	}{
		{200, false},
		{201, false},
		{400, false},
		{401, false},
		{403, false},
		{404, false},
		{429, true},
		{500, true},
		{502, false},
		{503, false},
		{529, true},
	}
	for _, tt := range tests {
		got := shouldRetry(tt.code)
		if got != tt.want {
			t.Errorf("shouldRetry(%d) = %v, want %v", tt.code, got, tt.want)
		}
	}
}

func TestBackoffDelay(t *testing.T) {
	// Run multiple samples to test statistical properties.
	for attempt := 0; attempt < 5; attempt++ {
		maxExpected := float64(int(1) << attempt) // base * 2^attempt
		if maxExpected > 30.0 {
			maxExpected = 30.0
		}

		for i := 0; i < 100; i++ {
			d := backoffDelay(attempt)
			if d < 0 {
				t.Errorf("backoffDelay(%d) = %v, want >= 0", attempt, d)
			}
			if d > time.Duration(maxExpected*float64(time.Second)) {
				t.Errorf("backoffDelay(%d) = %v, want <= %v", attempt, d, time.Duration(maxExpected*float64(time.Second)))
			}
		}
	}
}

func TestBackoffDelayCapAt30Seconds(t *testing.T) {
	// Very high attempt number should still be capped at 30 seconds.
	for i := 0; i < 100; i++ {
		d := backoffDelay(100)
		if d > 30*time.Second {
			t.Errorf("backoffDelay(100) = %v, exceeds 30s cap", d)
		}
	}
}

func TestRetryAfterDelay(t *testing.T) {
	t.Run("missing header", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{}}
		if d := retryAfterDelay(resp); d != 0 {
			t.Errorf("got %v, want 0", d)
		}
	})

	t.Run("integer seconds", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Set("Retry-After", "5")
		if d := retryAfterDelay(resp); d != 5*time.Second {
			t.Errorf("got %v, want 5s", d)
		}
	})

	t.Run("zero seconds", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Set("Retry-After", "0")
		if d := retryAfterDelay(resp); d != 0 {
			t.Errorf("got %v, want 0", d)
		}
	})

	t.Run("http-date in future", func(t *testing.T) {
		future := time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat)
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Set("Retry-After", future)
		d := retryAfterDelay(resp)
		// Should be roughly 10 seconds (allow some tolerance).
		if d < 8*time.Second || d > 12*time.Second {
			t.Errorf("got %v, want ~10s", d)
		}
	})

	t.Run("http-date in past", func(t *testing.T) {
		past := time.Now().Add(-10 * time.Second).UTC().Format(http.TimeFormat)
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Set("Retry-After", past)
		if d := retryAfterDelay(resp); d != 0 {
			t.Errorf("got %v, want 0 for past date", d)
		}
	})

	t.Run("unparseable value", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Set("Retry-After", "not-a-number")
		if d := retryAfterDelay(resp); d != 0 {
			t.Errorf("got %v, want 0 for unparseable", d)
		}
	})
}

func TestSleepWithContext(t *testing.T) {
	t.Run("completes normally", func(t *testing.T) {
		ctx := context.Background()
		start := time.Now()
		err := sleepWithContext(ctx, 10*time.Millisecond)
		elapsed := time.Since(start)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if elapsed < 10*time.Millisecond {
			t.Errorf("returned too early: %v", elapsed)
		}
	})

	t.Run("cancelled before sleep completes", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		// Cancel after a very short time.
		go func() {
			time.Sleep(5 * time.Millisecond)
			cancel()
		}()
		start := time.Now()
		err := sleepWithContext(ctx, 10*time.Second)
		elapsed := time.Since(start)
		if err != context.Canceled {
			t.Errorf("got err=%v, want context.Canceled", err)
		}
		if elapsed > 1*time.Second {
			t.Errorf("took too long to cancel: %v", elapsed)
		}
	})

	t.Run("already cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := sleepWithContext(ctx, 10*time.Second)
		if err != context.Canceled {
			t.Errorf("got err=%v, want context.Canceled", err)
		}
	})
}

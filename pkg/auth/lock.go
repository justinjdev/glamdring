package auth

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"time"
)

const (
	lockRetries    = 5
	lockBackoffMin = 1 * time.Second
	lockBackoffMax = 2 * time.Second
)

// lockPath returns the path to ~/.claude/.auth-lock.
func lockPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", ".auth-lock")
}

// AuthLock is a simple file-based lock for coordinating token refresh.
type AuthLock struct {
	path string
	f    *os.File
}

// AcquireLock attempts to acquire the auth lock file, retrying with random backoff.
func AcquireLock() (*AuthLock, error) {
	path := lockPath()

	// Ensure the directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}

	for attempt := range lockRetries {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			return &AuthLock{path: path, f: f}, nil
		}

		if !os.IsExist(err) {
			return nil, fmt.Errorf("open lock file: %w", err)
		}

		// Check if the lock is stale (older than 30 seconds).
		if info, statErr := os.Stat(path); statErr == nil {
			if time.Since(info.ModTime()) > 30*time.Second {
				// Stale lock — remove and retry immediately.
				os.Remove(path)
				continue
			}
		}

		if attempt < lockRetries-1 {
			jitter := lockBackoffMin + time.Duration(rand.Int64N(int64(lockBackoffMax-lockBackoffMin)))
			time.Sleep(jitter)
		}
	}

	return nil, fmt.Errorf("could not acquire auth lock after %d retries", lockRetries)
}

// Release releases the auth lock.
func (l *AuthLock) Release() {
	if l.f != nil {
		l.f.Close()
	}
	os.Remove(l.path)
}

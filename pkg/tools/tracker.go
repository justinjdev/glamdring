package tools

import (
	"path/filepath"
	"sync"
)

// ReadTracker records which file paths have been read during a session.
// It is shared between ReadTool and WriteTool to enforce read-before-write safety.
type ReadTracker struct {
	mu    sync.Mutex
	files map[string]bool
}

// NewReadTracker creates a new ReadTracker.
func NewReadTracker() *ReadTracker {
	return &ReadTracker{files: make(map[string]bool)}
}

// Record marks a file path as having been read.
func (t *ReadTracker) Record(path string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.files[filepath.Clean(path)] = true
}

// HasRead returns whether a file path has been recorded as read.
func (t *ReadTracker) HasRead(path string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.files[filepath.Clean(path)]
}

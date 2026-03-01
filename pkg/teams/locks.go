package teams

import (
	"fmt"
	"sync"
)

// LockEntry records who holds a lock and which task it belongs to.
type LockEntry struct {
	Owner  string
	TaskID string
}

// InMemoryLockManager is an in-memory implementation of LockManager.
type InMemoryLockManager struct {
	mu    sync.Mutex
	locks map[string]LockEntry // path -> entry
}

// NewInMemoryLockManager creates a new InMemoryLockManager.
func NewInMemoryLockManager() *InMemoryLockManager {
	return &InMemoryLockManager{
		locks: make(map[string]LockEntry),
	}
}

// Acquire locks a path for the given owner. Returns an error if the path is
// already locked by a different owner. Re-acquiring a lock by the same owner
// is a no-op.
func (m *InMemoryLockManager) Acquire(path string, owner string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, locked := m.locks[path]
	if locked && current.Owner != owner {
		return fmt.Errorf("path %q is locked by %q", path, current.Owner)
	}
	m.locks[path] = LockEntry{Owner: owner}
	return nil
}

// AcquireForTask locks a path for the given owner and task. Returns an error
// if the path is already locked by a different owner.
func (m *InMemoryLockManager) AcquireForTask(path, owner, taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, locked := m.locks[path]
	if locked && current.Owner != owner {
		return fmt.Errorf("path %q is locked by %q", path, current.Owner)
	}
	m.locks[path] = LockEntry{Owner: owner, TaskID: taskID}
	return nil
}

// Release unlocks a path. Returns an error if the path is not locked or is
// locked by a different owner.
func (m *InMemoryLockManager) Release(path string, owner string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, locked := m.locks[path]
	if !locked {
		return fmt.Errorf("path %q is not locked", path)
	}
	if current.Owner != owner {
		return fmt.Errorf("path %q is locked by %q, not %q", path, current.Owner, owner)
	}
	delete(m.locks, path)
	return nil
}

// ReleaseByTask releases all locks with the given task ID.
func (m *InMemoryLockManager) ReleaseByTask(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for path, entry := range m.locks {
		if entry.TaskID == taskID {
			delete(m.locks, path)
		}
	}
}

// Check returns the owner of a lock and whether the path is locked.
func (m *InMemoryLockManager) Check(path string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, locked := m.locks[path]
	return entry.Owner, locked
}

// ListLocks returns a snapshot of all current locks.
func (m *InMemoryLockManager) ListLocks() map[string]LockEntry {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make(map[string]LockEntry, len(m.locks))
	for path, entry := range m.locks {
		out[path] = entry
	}
	return out
}

// ReleaseAll releases all locks held by the given owner.
func (m *InMemoryLockManager) ReleaseAll(owner string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for path, entry := range m.locks {
		if entry.Owner == owner {
			delete(m.locks, path)
		}
	}
}

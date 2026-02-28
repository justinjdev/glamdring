package teams

import (
	"fmt"
	"sync"
)

// InMemoryLockManager is an in-memory implementation of LockManager.
type InMemoryLockManager struct {
	mu    sync.Mutex
	locks map[string]string // path -> owner
}

// NewInMemoryLockManager creates a new InMemoryLockManager.
func NewInMemoryLockManager() *InMemoryLockManager {
	return &InMemoryLockManager{
		locks: make(map[string]string),
	}
}

// Acquire locks a path for the given owner. Returns an error if the path is
// already locked by a different owner. Re-acquiring a lock by the same owner
// is a no-op.
func (m *InMemoryLockManager) Acquire(path string, owner string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, locked := m.locks[path]
	if locked && current != owner {
		return fmt.Errorf("path %q is locked by %q", path, current)
	}
	m.locks[path] = owner
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
	if current != owner {
		return fmt.Errorf("path %q is locked by %q, not %q", path, current, owner)
	}
	delete(m.locks, path)
	return nil
}

// Check returns the owner of a lock and whether the path is locked.
func (m *InMemoryLockManager) Check(path string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	owner, locked := m.locks[path]
	return owner, locked
}

// ReleaseAll releases all locks held by the given owner.
func (m *InMemoryLockManager) ReleaseAll(owner string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for path, o := range m.locks {
		if o == owner {
			delete(m.locks, path)
		}
	}
}

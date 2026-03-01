package teams

import (
	"fmt"
	"sync"
	"testing"
)

func TestLockManager_AcquireAndRelease(t *testing.T) {
	m := NewInMemoryLockManager()

	if err := m.Acquire("file.go", "alice"); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	owner, locked := m.Check("file.go")
	if !locked {
		t.Error("expected locked")
	}
	if owner != "alice" {
		t.Errorf("expected owner alice, got %q", owner)
	}

	if err := m.Release("file.go", "alice"); err != nil {
		t.Fatalf("Release: %v", err)
	}

	_, locked = m.Check("file.go")
	if locked {
		t.Error("expected unlocked after release")
	}
}

func TestLockManager_AcquireSameOwnerNoop(t *testing.T) {
	m := NewInMemoryLockManager()
	m.Acquire("file.go", "alice")

	// Re-acquiring by the same owner should be a no-op.
	if err := m.Acquire("file.go", "alice"); err != nil {
		t.Fatalf("re-acquire by same owner should not error: %v", err)
	}
}

func TestLockManager_AcquireDifferentOwnerError(t *testing.T) {
	m := NewInMemoryLockManager()
	m.Acquire("file.go", "alice")

	err := m.Acquire("file.go", "bob")
	if err == nil {
		t.Fatal("expected error for different owner acquiring same path")
	}
}

func TestLockManager_ReleaseNotLocked(t *testing.T) {
	m := NewInMemoryLockManager()
	err := m.Release("file.go", "alice")
	if err == nil {
		t.Fatal("expected error for releasing unlocked path")
	}
}

func TestLockManager_ReleaseWrongOwner(t *testing.T) {
	m := NewInMemoryLockManager()
	m.Acquire("file.go", "alice")

	err := m.Release("file.go", "bob")
	if err == nil {
		t.Fatal("expected error for releasing with wrong owner")
	}
}

func TestLockManager_CheckUnlocked(t *testing.T) {
	m := NewInMemoryLockManager()
	owner, locked := m.Check("file.go")
	if locked {
		t.Error("expected unlocked")
	}
	if owner != "" {
		t.Errorf("expected empty owner, got %q", owner)
	}
}

func TestLockManager_ReleaseAll(t *testing.T) {
	m := NewInMemoryLockManager()
	m.Acquire("a.go", "alice")
	m.Acquire("b.go", "alice")
	m.Acquire("c.go", "bob")

	m.ReleaseAll("alice")

	_, aLocked := m.Check("a.go")
	_, bLocked := m.Check("b.go")
	_, cLocked := m.Check("c.go")

	if aLocked {
		t.Error("a.go should be unlocked after ReleaseAll")
	}
	if bLocked {
		t.Error("b.go should be unlocked after ReleaseAll")
	}
	if !cLocked {
		t.Error("c.go should still be locked (owned by bob)")
	}
}

func TestInMemoryLockManager_Concurrent(t *testing.T) {
	m := NewInMemoryLockManager()
	const n = 20
	var wg sync.WaitGroup

	// Concurrent acquires on different paths.
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			m.Acquire(fmt.Sprintf("file-%d.go", i), fmt.Sprintf("agent-%d", i))
		}(i)
	}
	wg.Wait()

	// Concurrent checks.
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			owner, locked := m.Check(fmt.Sprintf("file-%d.go", i))
			if !locked {
				t.Errorf("file-%d.go should be locked", i)
			}
			if owner != fmt.Sprintf("agent-%d", i) {
				t.Errorf("file-%d.go: expected agent-%d, got %q", i, i, owner)
			}
		}(i)
	}
	wg.Wait()

	// Concurrent releases.
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			m.Release(fmt.Sprintf("file-%d.go", i), fmt.Sprintf("agent-%d", i))
		}(i)
	}
	wg.Wait()
}

func TestLockManager_AcquireForTask(t *testing.T) {
	m := NewInMemoryLockManager()

	if err := m.AcquireForTask("file.go", "alice", "task-1"); err != nil {
		t.Fatalf("AcquireForTask: %v", err)
	}

	owner, locked := m.Check("file.go")
	if !locked {
		t.Error("expected locked")
	}
	if owner != "alice" {
		t.Errorf("expected owner alice, got %q", owner)
	}

	// Same owner re-acquire should succeed.
	if err := m.AcquireForTask("file.go", "alice", "task-1"); err != nil {
		t.Fatalf("re-acquire by same owner should not error: %v", err)
	}

	// Different owner should fail.
	err := m.AcquireForTask("file.go", "bob", "task-2")
	if err == nil {
		t.Fatal("expected error for different owner acquiring same path")
	}
}

func TestLockManager_AcquireBackwardCompat(t *testing.T) {
	m := NewInMemoryLockManager()

	// Acquire without task ID should work and store empty TaskID.
	if err := m.Acquire("file.go", "alice"); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	// ReleaseByTask with empty string should not release it (Acquire stores
	// empty TaskID, but ReleaseByTask("") would match -- this tests that
	// normal Acquire locks are not accidentally released by a specific task).
	m.ReleaseByTask("task-1")

	_, locked := m.Check("file.go")
	if !locked {
		t.Error("expected lock to remain after ReleaseByTask with non-matching taskID")
	}
}

func TestLockManager_ReleaseByTask(t *testing.T) {
	m := NewInMemoryLockManager()

	m.AcquireForTask("a.go", "alice", "task-1")
	m.AcquireForTask("b.go", "alice", "task-1")
	m.AcquireForTask("c.go", "alice", "task-2")
	m.Acquire("d.go", "alice") // no task ID

	m.ReleaseByTask("task-1")

	_, aLocked := m.Check("a.go")
	_, bLocked := m.Check("b.go")
	_, cLocked := m.Check("c.go")
	_, dLocked := m.Check("d.go")

	if aLocked {
		t.Error("a.go should be unlocked after ReleaseByTask")
	}
	if bLocked {
		t.Error("b.go should be unlocked after ReleaseByTask")
	}
	if !cLocked {
		t.Error("c.go should still be locked (different task)")
	}
	if !dLocked {
		t.Error("d.go should still be locked (no task ID)")
	}
}

func TestLockManager_ReleaseByTaskConcurrent(t *testing.T) {
	m := NewInMemoryLockManager()
	const n = 20
	var wg sync.WaitGroup

	// Acquire locks for two tasks concurrently.
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			taskID := "task-1"
			if i%2 == 0 {
				taskID = "task-2"
			}
			m.AcquireForTask(fmt.Sprintf("file-%d.go", i), "alice", taskID)
		}(i)
	}
	wg.Wait()

	m.ReleaseByTask("task-1")

	for i := range n {
		_, locked := m.Check(fmt.Sprintf("file-%d.go", i))
		if i%2 == 0 {
			// task-2 locks should remain.
			if !locked {
				t.Errorf("file-%d.go (task-2) should still be locked", i)
			}
		} else {
			// task-1 locks should be released.
			if locked {
				t.Errorf("file-%d.go (task-1) should be unlocked", i)
			}
		}
	}
}

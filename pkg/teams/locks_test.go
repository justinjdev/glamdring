package teams

import "testing"

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

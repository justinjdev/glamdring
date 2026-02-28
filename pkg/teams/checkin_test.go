package teams

import "testing"

func TestCheckinTracker_Increment(t *testing.T) {
	ct := NewInMemoryCheckinTracker()

	got := ct.Increment("alice")
	if got != 1 {
		t.Errorf("expected 1, got %d", got)
	}

	got = ct.Increment("alice")
	if got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
}

func TestCheckinTracker_Count(t *testing.T) {
	ct := NewInMemoryCheckinTracker()

	if ct.Count("alice") != 0 {
		t.Errorf("expected 0 for new agent")
	}

	ct.Increment("alice")
	ct.Increment("alice")
	ct.Increment("alice")

	if ct.Count("alice") != 3 {
		t.Errorf("expected 3, got %d", ct.Count("alice"))
	}
}

func TestCheckinTracker_Reset(t *testing.T) {
	ct := NewInMemoryCheckinTracker()

	ct.Increment("alice")
	ct.Increment("alice")
	ct.Reset("alice")

	if ct.Count("alice") != 0 {
		t.Errorf("expected 0 after reset, got %d", ct.Count("alice"))
	}
}

func TestCheckinTracker_Remove(t *testing.T) {
	ct := NewInMemoryCheckinTracker()

	ct.Increment("alice")
	ct.Remove("alice")

	// After removal, count should be 0 (default for missing key).
	if ct.Count("alice") != 0 {
		t.Errorf("expected 0 after remove, got %d", ct.Count("alice"))
	}
}

func TestCheckinTracker_MultipleAgents(t *testing.T) {
	ct := NewInMemoryCheckinTracker()

	ct.Increment("alice")
	ct.Increment("alice")
	ct.Increment("bob")

	if ct.Count("alice") != 2 {
		t.Errorf("expected alice=2, got %d", ct.Count("alice"))
	}
	if ct.Count("bob") != 1 {
		t.Errorf("expected bob=1, got %d", ct.Count("bob"))
	}
}

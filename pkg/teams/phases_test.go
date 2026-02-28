package teams

import "testing"

func testPhases() []Phase {
	return []Phase{
		{Name: "research", Tools: []string{"Read"}, Model: "sonnet"},
		{Name: "plan", Tools: []string{"Read", "Glob"}, Model: "sonnet"},
		{Name: "implement", Tools: []string{"Read", "Write"}, Model: "sonnet"},
	}
}

func TestPhaseTracker_SetPhasesAndCurrent(t *testing.T) {
	pt := NewInMemoryPhaseTracker()
	pt.SetPhases("alice", testPhases())

	phase, idx, err := pt.Current("alice")
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if idx != 0 {
		t.Errorf("expected index 0, got %d", idx)
	}
	if phase.Name != "research" {
		t.Errorf("expected phase 'research', got %q", phase.Name)
	}
}

func TestPhaseTracker_CurrentNotFound(t *testing.T) {
	pt := NewInMemoryPhaseTracker()
	_, _, err := pt.Current("nobody")
	if err == nil {
		t.Fatal("expected error for non-existent agent")
	}
}

func TestPhaseTracker_Advance(t *testing.T) {
	pt := NewInMemoryPhaseTracker()
	pt.SetPhases("alice", testPhases())

	phase, err := pt.Advance("alice")
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if phase.Name != "plan" {
		t.Errorf("expected 'plan', got %q", phase.Name)
	}

	phase, err = pt.Advance("alice")
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if phase.Name != "implement" {
		t.Errorf("expected 'implement', got %q", phase.Name)
	}
}

func TestPhaseTracker_AdvancePastEnd(t *testing.T) {
	pt := NewInMemoryPhaseTracker()
	pt.SetPhases("alice", testPhases())

	pt.Advance("alice")
	pt.Advance("alice")

	_, err := pt.Advance("alice")
	if err == nil {
		t.Fatal("expected error for advancing past last phase")
	}
}

func TestPhaseTracker_AdvanceNotFound(t *testing.T) {
	pt := NewInMemoryPhaseTracker()
	_, err := pt.Advance("nobody")
	if err == nil {
		t.Fatal("expected error for non-existent agent")
	}
}

func TestPhaseTracker_AdvanceTo(t *testing.T) {
	pt := NewInMemoryPhaseTracker()
	pt.SetPhases("alice", testPhases())

	phase, err := pt.AdvanceTo("alice", "implement")
	if err != nil {
		t.Fatalf("AdvanceTo: %v", err)
	}
	if phase.Name != "implement" {
		t.Errorf("expected 'implement', got %q", phase.Name)
	}

	// Verify current was updated.
	current, idx, _ := pt.Current("alice")
	if idx != 2 {
		t.Errorf("expected index 2, got %d", idx)
	}
	if current.Name != "implement" {
		t.Errorf("expected current 'implement', got %q", current.Name)
	}
}

func TestPhaseTracker_AdvanceToNonExistentPhase(t *testing.T) {
	pt := NewInMemoryPhaseTracker()
	pt.SetPhases("alice", testPhases())

	_, err := pt.AdvanceTo("alice", "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent phase name")
	}
}

func TestPhaseTracker_AdvanceToNonExistentAgent(t *testing.T) {
	pt := NewInMemoryPhaseTracker()
	_, err := pt.AdvanceTo("nobody", "research")
	if err == nil {
		t.Fatal("expected error for non-existent agent")
	}
}

func TestPhaseTracker_Remove(t *testing.T) {
	pt := NewInMemoryPhaseTracker()
	pt.SetPhases("alice", testPhases())
	pt.Remove("alice")

	_, _, err := pt.Current("alice")
	if err == nil {
		t.Error("expected error after removal")
	}
}

func TestPhaseTracker_RemoveNonExistent(t *testing.T) {
	pt := NewInMemoryPhaseTracker()
	// Should not panic.
	pt.Remove("nobody")
}

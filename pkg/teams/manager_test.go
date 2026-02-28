package teams

import "testing"

func TestNewTeamManager(t *testing.T) {
	dir := t.TempDir()
	cfg := TeamConfig{Name: "test-team"}

	mgr, err := NewTeamManager(cfg, dir)
	if err != nil {
		t.Fatalf("NewTeamManager: %v", err)
	}

	if mgr.Config.Name != "test-team" {
		t.Errorf("expected config name 'test-team', got %q", mgr.Config.Name)
	}
	if mgr.Members == nil {
		t.Error("Members is nil")
	}
	if mgr.Tasks == nil {
		t.Error("Tasks is nil")
	}
	if mgr.Messages == nil {
		t.Error("Messages is nil")
	}
	if mgr.Locks == nil {
		t.Error("Locks is nil")
	}
	if mgr.Context == nil {
		t.Error("Context is nil")
	}
	if mgr.Phases == nil {
		t.Error("Phases is nil")
	}
	if mgr.Checkins == nil {
		t.Error("Checkins is nil")
	}
}

func TestTeamManager_CleanupAgent(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewTeamManager(TeamConfig{Name: "test"}, dir)

	// Set up a member with various subsystem state.
	mgr.Members.Add(Member{Name: "alice", Status: MemberStatusActive})
	mgr.Messages.Subscribe("alice", 10)
	mgr.Locks.Acquire("file.go", "alice")
	mgr.Phases.SetPhases("alice", testPhases())
	mgr.Checkins.Increment("alice")

	if err := mgr.CleanupAgent("alice"); err != nil {
		t.Fatalf("CleanupAgent: %v", err)
	}

	// Member should be shutdown.
	m, _ := mgr.Members.Get("alice")
	if m.Status != MemberStatusShutdown {
		t.Errorf("expected shutdown status, got %q", m.Status)
	}

	// Lock should be released.
	_, locked := mgr.Locks.Check("file.go")
	if locked {
		t.Error("lock should be released after cleanup")
	}

	// Phase tracker should not find the agent.
	_, _, err := mgr.Phases.Current("alice")
	if err == nil {
		t.Error("expected error from phase tracker after cleanup")
	}

	// Checkin should be removed.
	if mgr.Checkins.Count("alice") != 0 {
		t.Error("checkin count should be 0 after cleanup")
	}
}

func TestTeamManager_CleanupAgentNotFound(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewTeamManager(TeamConfig{Name: "test"}, dir)

	err := mgr.CleanupAgent("nobody")
	if err == nil {
		t.Fatal("expected error for non-existent agent cleanup")
	}
}

func TestTeamManager_DeleteWithActiveMembers(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewTeamManager(TeamConfig{Name: "test"}, dir)

	mgr.Members.Add(Member{Name: "alice", Status: MemberStatusActive})

	err := mgr.Delete()
	if err == nil {
		t.Fatal("expected error when deleting team with active members")
	}
}

func TestTeamManager_DeleteNoActiveMembers(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewTeamManager(TeamConfig{Name: "test"}, dir)

	// Add a member that's already shutdown.
	mgr.Members.Add(Member{Name: "alice", Status: MemberStatusShutdown})

	if err := mgr.Delete(); err != nil {
		t.Fatalf("Delete should succeed with no active members: %v", err)
	}
}

func TestTeamManager_DeleteEmptyTeam(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewTeamManager(TeamConfig{Name: "test"}, dir)

	if err := mgr.Delete(); err != nil {
		t.Fatalf("Delete should succeed with empty team: %v", err)
	}
}

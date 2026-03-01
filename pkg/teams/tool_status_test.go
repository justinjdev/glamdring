package teams

import (
	"context"
	"encoding/json"
	"testing"
)

func setupStatusTestTeam(t *testing.T) (*ManagerRegistry, *TeamManager) {
	t.Helper()
	reg := NewManagerRegistry()
	dir := t.TempDir()
	mgr, err := reg.Create(TeamConfig{Name: "status-team"}, dir)
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	return reg, mgr
}

func TestTeamStatusTool_BasicOutput(t *testing.T) {
	reg, mgr := setupStatusTestTeam(t)

	// Add members.
	mgr.Members.Add(Member{Name: "alice", Status: MemberStatusActive})
	mgr.Members.Add(Member{Name: "bob", Status: MemberStatusIdle})

	// Acquire some locks.
	mgr.Locks.AcquireForTask("/src/main.go", "alice", "1")
	mgr.Locks.Acquire("/src/util.go", "bob")

	// Create tasks.
	mgr.Tasks.Create(Task{Subject: "task 1", Status: TaskStatusPending})
	mgr.Tasks.Create(Task{Subject: "task 2", Status: TaskStatusInProgress})
	mgr.Tasks.Create(Task{Subject: "task 3", Status: TaskStatusCompleted})

	// Set phases for alice.
	mgr.Phases.SetPhases("alice", []Phase{
		{Name: "research", Tools: []string{"Read"}},
		{Name: "implement", Tools: []string{"Write"}},
	})

	tool := TeamStatusTool{Registry: reg}
	input, _ := json.Marshal(map[string]string{"team_name": "status-team"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	var out teamStatusOutput
	if err := json.Unmarshal([]byte(result.Output), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if out.TeamName != "status-team" {
		t.Errorf("expected team_name 'status-team', got %q", out.TeamName)
	}
	if len(out.Members) != 2 {
		t.Errorf("expected 2 members, got %d", len(out.Members))
	}
	if len(out.Locks) != 2 {
		t.Errorf("expected 2 locks, got %d", len(out.Locks))
	}
	if out.Tasks.Total != 3 {
		t.Errorf("expected 3 total tasks, got %d", out.Tasks.Total)
	}
	if out.Tasks.Pending != 1 {
		t.Errorf("expected 1 pending, got %d", out.Tasks.Pending)
	}
	if out.Tasks.InProgress != 1 {
		t.Errorf("expected 1 in_progress, got %d", out.Tasks.InProgress)
	}
	if out.Tasks.Completed != 1 {
		t.Errorf("expected 1 completed, got %d", out.Tasks.Completed)
	}
	if len(out.Phases) != 1 {
		t.Errorf("expected 1 phase entry, got %d", len(out.Phases))
	}
	if len(out.Phases) > 0 {
		if out.Phases[0].Agent != "alice" {
			t.Errorf("expected phase agent 'alice', got %q", out.Phases[0].Agent)
		}
		if out.Phases[0].Phase != "research" {
			t.Errorf("expected phase 'research', got %q", out.Phases[0].Phase)
		}
	}
}

func TestTeamStatusTool_EmptyTeam(t *testing.T) {
	reg, _ := setupStatusTestTeam(t)

	tool := TeamStatusTool{Registry: reg}
	input, _ := json.Marshal(map[string]string{"team_name": "status-team"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	var out teamStatusOutput
	json.Unmarshal([]byte(result.Output), &out)

	if len(out.Members) != 0 {
		t.Errorf("expected 0 members, got %d", len(out.Members))
	}
	if len(out.Locks) != 0 {
		t.Errorf("expected 0 locks, got %d", len(out.Locks))
	}
	if out.Tasks.Total != 0 {
		t.Errorf("expected 0 tasks, got %d", out.Tasks.Total)
	}
}

func TestTeamStatusTool_TeamNotFound(t *testing.T) {
	reg := NewManagerRegistry()
	tool := TeamStatusTool{Registry: reg}

	input, _ := json.Marshal(map[string]string{"team_name": "nope"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for non-existent team")
	}
}

func TestTeamStatusTool_LockEntryDetails(t *testing.T) {
	reg, mgr := setupStatusTestTeam(t)

	mgr.Locks.AcquireForTask("/src/main.go", "alice", "42")

	tool := TeamStatusTool{Registry: reg}
	input, _ := json.Marshal(map[string]string{"team_name": "status-team"})
	result, _ := tool.Execute(context.Background(), input)

	var out teamStatusOutput
	json.Unmarshal([]byte(result.Output), &out)

	if len(out.Locks) != 1 {
		t.Fatalf("expected 1 lock, got %d", len(out.Locks))
	}
	lock := out.Locks[0]
	if lock.Path != "/src/main.go" {
		t.Errorf("expected path '/src/main.go', got %q", lock.Path)
	}
	if lock.Owner != "alice" {
		t.Errorf("expected owner 'alice', got %q", lock.Owner)
	}
	if lock.TaskID != "42" {
		t.Errorf("expected taskID '42', got %q", lock.TaskID)
	}
}

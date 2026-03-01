package teams

import (
	"context"
	"encoding/json"
	"testing"
)

func setupTaskTestTeam(t *testing.T) *ManagerRegistry {
	t.Helper()
	reg := NewManagerRegistry()
	dir := t.TempDir()
	_, err := reg.Create(TeamConfig{Name: "proj"}, dir)
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	return reg
}

func TestTaskCreateTool_Creates(t *testing.T) {
	reg := setupTaskTestTeam(t)
	tool := TaskCreateTool{Registry: reg}

	input, _ := json.Marshal(map[string]string{
		"team_name":   "proj",
		"subject":     "implement feature X",
		"description": "detailed description",
		"owner":       "alice",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	var out map[string]string
	json.Unmarshal([]byte(result.Output), &out)
	if out["task_id"] == "" {
		t.Error("expected non-empty task_id")
	}
	if out["subject"] != "implement feature X" {
		t.Errorf("expected subject 'implement feature X', got %q", out["subject"])
	}
}

func TestTaskCreateTool_MissingSubject(t *testing.T) {
	reg := setupTaskTestTeam(t)
	tool := TaskCreateTool{Registry: reg}

	input, _ := json.Marshal(map[string]string{"team_name": "proj"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing subject")
	}
}

func TestTaskCreateTool_TeamNotFound(t *testing.T) {
	reg := NewManagerRegistry()
	tool := TaskCreateTool{Registry: reg}

	input, _ := json.Marshal(map[string]string{"team_name": "nope", "subject": "test"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for non-existent team")
	}
}

func TestTaskListTool_Lists(t *testing.T) {
	reg := setupTaskTestTeam(t)
	createTool := TaskCreateTool{Registry: reg}
	listTool := TaskListTool{Registry: reg}

	// Create two tasks.
	for _, subj := range []string{"task A", "task B"} {
		input, _ := json.Marshal(map[string]string{"team_name": "proj", "subject": subj})
		createTool.Execute(context.Background(), input)
	}

	listInput, _ := json.Marshal(map[string]string{"team_name": "proj"})
	result, err := listTool.Execute(context.Background(), listInput)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	var summaries []TaskSummary
	json.Unmarshal([]byte(result.Output), &summaries)
	if len(summaries) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(summaries))
	}
}

func TestTaskGetTool_Gets(t *testing.T) {
	reg := setupTaskTestTeam(t)
	createTool := TaskCreateTool{Registry: reg}
	getTool := TaskGetTool{Registry: reg}

	createInput, _ := json.Marshal(map[string]string{"team_name": "proj", "subject": "find me"})
	createResult, _ := createTool.Execute(context.Background(), createInput)

	var created map[string]string
	json.Unmarshal([]byte(createResult.Output), &created)

	getInput, _ := json.Marshal(map[string]string{"team_name": "proj", "task_id": created["task_id"]})
	result, err := getTool.Execute(context.Background(), getInput)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	var task Task
	json.Unmarshal([]byte(result.Output), &task)
	if task.Subject != "find me" {
		t.Errorf("expected subject 'find me', got %q", task.Subject)
	}
}

func TestTaskGetTool_NotFound(t *testing.T) {
	reg := setupTaskTestTeam(t)
	tool := TaskGetTool{Registry: reg}

	input, _ := json.Marshal(map[string]string{"team_name": "proj", "task_id": "999"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for non-existent task")
	}
}

func TestTaskUpdateTool_UpdatesStatus(t *testing.T) {
	reg := setupTaskTestTeam(t)
	createTool := TaskCreateTool{Registry: reg}
	updateTool := TaskUpdateTool{Registry: reg, AgentName: "alice"}

	createInput, _ := json.Marshal(map[string]string{"team_name": "proj", "subject": "update me"})
	createResult, _ := createTool.Execute(context.Background(), createInput)

	var created map[string]string
	json.Unmarshal([]byte(createResult.Output), &created)

	updateInput, _ := json.Marshal(map[string]any{
		"team_name": "proj",
		"task_id":   created["task_id"],
		"status":    "in_progress",
		"owner":     "alice",
	})
	result, err := updateTool.Execute(context.Background(), updateInput)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	var task Task
	json.Unmarshal([]byte(result.Output), &task)
	if task.Status != TaskStatusInProgress {
		t.Errorf("expected status 'in_progress', got %q", task.Status)
	}
	if task.Owner != "alice" {
		t.Errorf("expected owner 'alice', got %q", task.Owner)
	}
}

func TestTaskUpdateTool_CASOwnership(t *testing.T) {
	reg := setupTaskTestTeam(t)
	createTool := TaskCreateTool{Registry: reg}
	updateTool := TaskUpdateTool{Registry: reg, AgentName: "alice"}

	createInput, _ := json.Marshal(map[string]string{"team_name": "proj", "subject": "cas test", "owner": "alice"})
	createResult, _ := createTool.Execute(context.Background(), createInput)

	var created map[string]string
	json.Unmarshal([]byte(createResult.Output), &created)

	// Try to update with wrong expected_owner.
	updateInput, _ := json.Marshal(map[string]any{
		"team_name":      "proj",
		"task_id":        created["task_id"],
		"owner":          "bob",
		"expected_owner": "charlie",
	})
	result, err := updateTool.Execute(context.Background(), updateInput)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for CAS ownership conflict")
	}

	// Update with correct expected_owner.
	updateInput, _ = json.Marshal(map[string]any{
		"team_name":      "proj",
		"task_id":        created["task_id"],
		"owner":          "bob",
		"expected_owner": "alice",
	})
	result, err = updateTool.Execute(context.Background(), updateInput)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	var task Task
	json.Unmarshal([]byte(result.Output), &task)
	if task.Owner != "bob" {
		t.Errorf("expected owner 'bob', got %q", task.Owner)
	}
}

func TestTaskUpdateTool_ResetsCheckin(t *testing.T) {
	reg := setupTaskTestTeam(t)
	createTool := TaskCreateTool{Registry: reg}
	updateTool := TaskUpdateTool{Registry: reg, AgentName: "alice"}

	mgr := reg.Get("proj")
	mgr.Checkins.Increment("alice")
	mgr.Checkins.Increment("alice")
	if mgr.Checkins.Count("alice") != 2 {
		t.Fatalf("expected checkin count 2, got %d", mgr.Checkins.Count("alice"))
	}

	createInput, _ := json.Marshal(map[string]string{"team_name": "proj", "subject": "checkin test"})
	createResult, _ := createTool.Execute(context.Background(), createInput)
	var created map[string]string
	json.Unmarshal([]byte(createResult.Output), &created)

	updateInput, _ := json.Marshal(map[string]any{
		"team_name": "proj",
		"task_id":   created["task_id"],
		"status":    "in_progress",
	})
	result, _ := updateTool.Execute(context.Background(), updateInput)
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	if mgr.Checkins.Count("alice") != 0 {
		t.Errorf("expected checkin count 0 after reset, got %d", mgr.Checkins.Count("alice"))
	}
}

func TestTaskUpdateTool_CompletionReleasesLocks(t *testing.T) {
	reg := setupTaskTestTeam(t)
	createTool := TaskCreateTool{Registry: reg}
	updateTool := TaskUpdateTool{Registry: reg, AgentName: "alice"}

	// Create a task and get its ID.
	createInput, _ := json.Marshal(map[string]string{"team_name": "proj", "subject": "lock test"})
	createResult, _ := createTool.Execute(context.Background(), createInput)
	var created map[string]string
	json.Unmarshal([]byte(createResult.Output), &created)
	taskID := created["task_id"]

	// Acquire locks tagged with this task ID.
	mgr := reg.Get("proj")
	mgr.Locks.AcquireForTask("/project/a.go", "alice", taskID)
	mgr.Locks.AcquireForTask("/project/b.go", "alice", taskID)
	// A lock for a different task should not be released.
	mgr.Locks.AcquireForTask("/project/c.go", "alice", "other-task")

	// Complete the task.
	updateInput, _ := json.Marshal(map[string]any{
		"team_name": "proj",
		"task_id":   taskID,
		"status":    "completed",
	})
	result, err := updateTool.Execute(context.Background(), updateInput)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	// Locks for the completed task should be released.
	_, aLocked := mgr.Locks.Check("/project/a.go")
	_, bLocked := mgr.Locks.Check("/project/b.go")
	_, cLocked := mgr.Locks.Check("/project/c.go")

	if aLocked {
		t.Error("a.go should be unlocked after task completion")
	}
	if bLocked {
		t.Error("b.go should be unlocked after task completion")
	}
	if !cLocked {
		t.Error("c.go should still be locked (different task)")
	}
}

func TestTaskUpdateTool_NonCompletionDoesNotReleaseLocks(t *testing.T) {
	reg := setupTaskTestTeam(t)
	createTool := TaskCreateTool{Registry: reg}
	updateTool := TaskUpdateTool{Registry: reg, AgentName: "alice"}

	createInput, _ := json.Marshal(map[string]string{"team_name": "proj", "subject": "no release test"})
	createResult, _ := createTool.Execute(context.Background(), createInput)
	var created map[string]string
	json.Unmarshal([]byte(createResult.Output), &created)
	taskID := created["task_id"]

	mgr := reg.Get("proj")
	mgr.Locks.AcquireForTask("/project/a.go", "alice", taskID)

	// Set to in_progress (not completed).
	updateInput, _ := json.Marshal(map[string]any{
		"team_name": "proj",
		"task_id":   taskID,
		"status":    "in_progress",
	})
	result, _ := updateTool.Execute(context.Background(), updateInput)
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	// Lock should still be held.
	_, locked := mgr.Locks.Check("/project/a.go")
	if !locked {
		t.Error("lock should remain when task is not completed")
	}
}

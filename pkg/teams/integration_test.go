package teams

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/justin/glamdring/pkg/tools"
)

// TestIntegration_FullLifecycle exercises the full team lifecycle:
// create team -> add member -> create tasks -> update task -> send message
// -> advance phase -> cleanup -> delete team.
func TestIntegration_FullLifecycle(t *testing.T) {
	registry := NewManagerRegistry()
	taskDir := t.TempDir()

	// Create a team with RPIV workflow.
	cfg := TeamConfig{
		Name:     "test-team",
		Workflow: "rpiv",
		Phases:   WorkflowRPIV(),
	}
	mgr, err := registry.Create(cfg, taskDir)
	if err != nil {
		t.Fatalf("create team: %v", err)
	}

	// Add a member.
	err = mgr.Members.Add(Member{Name: "agent-1", Status: MemberStatusActive})
	if err != nil {
		t.Fatalf("add member: %v", err)
	}

	// Subscribe member to messages.
	regularCh, priorityCh, err := mgr.Messages.Subscribe("agent-1", 16)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// Set up phases for the agent.
	mgr.Phases.SetPhases("agent-1", cfg.Phases)

	// Verify initial phase is "research".
	phase, idx, err := mgr.Phases.Current("agent-1")
	if err != nil {
		t.Fatalf("get current phase: %v", err)
	}
	if phase.Name != "research" {
		t.Errorf("expected research phase, got %s", phase.Name)
	}
	if idx != 0 {
		t.Errorf("expected index 0, got %d", idx)
	}

	// Create tasks.
	task1, err := mgr.Tasks.Create(Task{Subject: "Research codebase", Status: TaskStatusPending})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	task2, err := mgr.Tasks.Create(Task{
		Subject:   "Implement feature",
		Status:    TaskStatusPending,
		BlockedBy: []string{task1.ID},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Verify task2 is blocked by task1.
	summaries := mgr.Tasks.List()
	if len(summaries) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(summaries))
	}
	for _, s := range summaries {
		if s.ID == task2.ID {
			if len(s.BlockedBy) != 1 || s.BlockedBy[0] != task1.ID {
				t.Errorf("task2 should be blocked by task1")
			}
		}
	}

	// Update task1: assign owner and complete it.
	owner := "agent-1"
	_, err = mgr.Tasks.Update(task1.ID, TaskUpdate{Owner: &owner})
	if err != nil {
		t.Fatalf("assign task: %v", err)
	}

	completed := TaskStatusCompleted
	_, err = mgr.Tasks.Update(task1.ID, TaskUpdate{Status: &completed})
	if err != nil {
		t.Fatalf("complete task: %v", err)
	}

	// Verify task2 is no longer blocked.
	task2Updated, err := mgr.Tasks.Get(task2.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if len(task2Updated.BlockedBy) != 0 {
		t.Errorf("expected task2 to be unblocked, got blockedBy=%v", task2Updated.BlockedBy)
	}

	// Send a message from lead to agent-1.
	err = mgr.Messages.Send(AgentMessage{
		Kind:    MessageKindDM,
		From:    "lead",
		To:      "agent-1",
		Content: "task1 is done, proceed with task2",
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}

	// Verify message was received on regular channel.
	select {
	case msg := <-regularCh:
		if msg.Content != "task1 is done, proceed with task2" {
			t.Errorf("unexpected message content: %s", msg.Content)
		}
	default:
		t.Error("expected message on regular channel")
	}

	// Send a shutdown request (should go to priority channel).
	err = mgr.Messages.Send(AgentMessage{
		Kind:    MessageKindShutdownRequest,
		From:    "lead",
		To:      "agent-1",
		Content: "work complete",
	})
	if err != nil {
		t.Fatalf("send shutdown: %v", err)
	}

	select {
	case msg := <-priorityCh:
		if msg.Kind != MessageKindShutdownRequest {
			t.Errorf("expected shutdown_request, got %s", msg.Kind)
		}
	default:
		t.Error("expected message on priority channel")
	}

	// Advance phase.
	newPhase, err := mgr.Phases.Advance("agent-1")
	if err != nil {
		t.Fatalf("advance phase: %v", err)
	}
	if newPhase.Name != "plan" {
		t.Errorf("expected plan phase, got %s", newPhase.Name)
	}

	// File locking.
	err = mgr.Locks.Acquire("/src/main.go", "agent-1")
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}
	lockOwner, locked := mgr.Locks.Check("/src/main.go")
	if !locked || lockOwner != "agent-1" {
		t.Errorf("expected lock by agent-1, got owner=%s locked=%v", lockOwner, locked)
	}

	// Checkin tracking.
	count := mgr.Checkins.Increment("agent-1")
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
	mgr.Checkins.Reset("agent-1")
	if mgr.Checkins.Count("agent-1") != 0 {
		t.Error("expected count 0 after reset")
	}

	// Context cache.
	mgr.Context.Store("research-summary", "Found 3 relevant files.")
	val, ok := mgr.Context.Load("research-summary")
	if !ok || val != "Found 3 relevant files." {
		t.Errorf("unexpected context cache value: %q", val)
	}

	// Cleanup agent.
	err = mgr.CleanupAgent("agent-1")
	if err != nil {
		t.Fatalf("cleanup agent: %v", err)
	}

	// Verify member is shutdown.
	member, err := mgr.Members.Get("agent-1")
	if err != nil {
		t.Fatalf("get member: %v", err)
	}
	if member.Status != MemberStatusShutdown {
		t.Errorf("expected shutdown status, got %s", member.Status)
	}

	// Locks should be released.
	_, locked = mgr.Locks.Check("/src/main.go")
	if locked {
		t.Error("expected lock to be released after cleanup")
	}

	// Delete team (should succeed since no active members).
	err = registry.Delete("test-team")
	if err != nil {
		t.Fatalf("delete team: %v", err)
	}

	// Verify team is gone.
	if registry.Get("test-team") != nil {
		t.Error("expected team to be deleted")
	}
}

// TestIntegration_ToolsEndToEnd tests the tool layer with real subsystems.
func TestIntegration_ToolsEndToEnd(t *testing.T) {
	registry := NewManagerRegistry()
	ctx := context.Background()

	// Create team via tool.
	createTool := TeamCreateTool{Registry: registry, TaskDirBase: t.TempDir()}
	input, _ := json.Marshal(map[string]string{
		"team_name":   "e2e-team",
		"description": "end to end test",
		"workflow":    "none",
	})
	result, err := createTool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("create tool: %v", err)
	}
	if result.IsError {
		t.Fatalf("create tool error: %s", result.Output)
	}

	// Create task via tool.
	taskCreate := TaskCreateTool{Registry: registry}
	input, _ = json.Marshal(map[string]string{
		"team_name": "e2e-team",
		"subject":   "implement feature X",
	})
	result, err = taskCreate.Execute(ctx, input)
	if err != nil {
		t.Fatalf("task create: %v", err)
	}
	if result.IsError {
		t.Fatalf("task create error: %s", result.Output)
	}

	var created map[string]string
	if err := json.Unmarshal([]byte(result.Output), &created); err != nil {
		t.Fatalf("parse create result: %v", err)
	}
	taskID := created["task_id"]

	// List tasks via tool.
	taskList := TaskListTool{Registry: registry}
	input, _ = json.Marshal(map[string]string{"team_name": "e2e-team"})
	result, err = taskList.Execute(ctx, input)
	if err != nil {
		t.Fatalf("task list: %v", err)
	}
	var summaries []TaskSummary
	if err := json.Unmarshal([]byte(result.Output), &summaries); err != nil {
		t.Fatalf("parse list result: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 task, got %d", len(summaries))
	}

	// Get task via tool.
	taskGet := TaskGetTool{Registry: registry}
	input, _ = json.Marshal(map[string]string{"team_name": "e2e-team", "task_id": taskID})
	result, err = taskGet.Execute(ctx, input)
	if err != nil {
		t.Fatalf("task get: %v", err)
	}
	var task Task
	if err := json.Unmarshal([]byte(result.Output), &task); err != nil {
		t.Fatalf("parse get result: %v", err)
	}
	if task.Subject != "implement feature X" {
		t.Errorf("expected subject 'implement feature X', got %q", task.Subject)
	}

	// Update task via tool.
	taskUpdate := TaskUpdateTool{Registry: registry, AgentName: "worker-1"}
	input, _ = json.Marshal(map[string]any{
		"team_name": "e2e-team",
		"task_id":   taskID,
		"status":    "in_progress",
		"owner":     "worker-1",
	})
	result, err = taskUpdate.Execute(ctx, input)
	if err != nil {
		t.Fatalf("task update: %v", err)
	}
	if result.IsError {
		t.Fatalf("task update error: %s", result.Output)
	}

	// Delete team via tool.
	deleteTool := TeamDeleteTool{Registry: registry}
	input, _ = json.Marshal(map[string]string{"team_name": "e2e-team"})
	result, err = deleteTool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("delete tool: %v", err)
	}
	if result.IsError {
		t.Fatalf("delete tool error: %s", result.Output)
	}
}

// TestIntegration_PhaseRegistry tests PhaseRegistry with real subsystems.
func TestIntegration_PhaseRegistry(t *testing.T) {
	tracker := NewInMemoryPhaseTracker()
	tracker.SetPhases("agent-1", WorkflowRPIV())

	base := tools.NewRegistry()
	base.Register(&stubToolForIntegration{name: "Read"})
	base.Register(&stubToolForIntegration{name: "Write"})
	base.Register(&stubToolForIntegration{name: "Edit"})
	base.Register(&stubToolForIntegration{name: "Glob"})
	base.Register(&stubToolForIntegration{name: "Grep"})
	base.Register(&stubToolForIntegration{name: "Bash"})
	base.Register(&stubToolForIntegration{name: "TaskCreate"})
	base.Register(&stubToolForIntegration{name: "SendMessage"})

	pr := NewPhaseRegistry(base, tracker, "agent-1", nil, nil)

	// Research phase: Write and Edit should not be available.
	schemas := pr.Schemas()
	schemaNames := make(map[string]bool)
	for _, s := range schemas {
		var parsed map[string]any
		json.Unmarshal(s, &parsed)
		schemaNames[parsed["name"].(string)] = true
	}
	if schemaNames["Write"] {
		t.Error("Write should not be available in research phase")
	}
	if schemaNames["Edit"] {
		t.Error("Edit should not be available in research phase")
	}
	if !schemaNames["Read"] {
		t.Error("Read should be available in research phase")
	}
	if !schemaNames["TaskCreate"] {
		t.Error("TaskCreate should be available in all phases")
	}

	// Advance to implement phase.
	tracker.Advance("agent-1")
	tracker.Advance("agent-1") // skip plan, go to implement

	schemas = pr.Schemas()
	schemaNames = make(map[string]bool)
	for _, s := range schemas {
		var parsed map[string]any
		json.Unmarshal(s, &parsed)
		schemaNames[parsed["name"].(string)] = true
	}
	if !schemaNames["Write"] {
		t.Error("Write should be available in implement phase")
	}
	if !schemaNames["Edit"] {
		t.Error("Edit should be available in implement phase")
	}
}

type stubToolForIntegration struct {
	name string
}

func (s *stubToolForIntegration) Name() string        { return s.name }
func (s *stubToolForIntegration) Description() string  { return "stub" }
func (s *stubToolForIntegration) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (s *stubToolForIntegration) Execute(_ context.Context, _ json.RawMessage) (tools.Result, error) {
	return tools.Result{Output: "ok"}, nil
}

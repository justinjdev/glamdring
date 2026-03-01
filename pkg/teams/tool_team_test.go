package teams

import (
	"context"
	"encoding/json"
	"testing"
)

func TestTeamCreateTool_Creates(t *testing.T) {
	reg := NewManagerRegistry()
	tool := TeamCreateTool{Registry: reg}

	input, _ := json.Marshal(map[string]string{"team_name": "test-team", "description": "a test"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	var out map[string]string
	if err := json.Unmarshal([]byte(result.Output), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out["team_name"] != "test-team" {
		t.Errorf("expected team_name 'test-team', got %q", out["team_name"])
	}

	if reg.Get("test-team") == nil {
		t.Error("team should exist in registry after create")
	}
}

func TestTeamCreateTool_DuplicateErrors(t *testing.T) {
	reg := NewManagerRegistry()
	tool := TeamCreateTool{Registry: reg}

	input, _ := json.Marshal(map[string]string{"team_name": "dup"})
	tool.Execute(context.Background(), input)

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for duplicate team create")
	}
}

func TestTeamCreateTool_MissingName(t *testing.T) {
	reg := NewManagerRegistry()
	tool := TeamCreateTool{Registry: reg}

	input, _ := json.Marshal(map[string]string{})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing team_name")
	}
}

func TestTeamDeleteTool_Deletes(t *testing.T) {
	reg := NewManagerRegistry()
	createTool := TeamCreateTool{Registry: reg}
	deleteTool := TeamDeleteTool{Registry: reg}

	createInput, _ := json.Marshal(map[string]string{"team_name": "to-delete"})
	createTool.Execute(context.Background(), createInput)

	deleteInput, _ := json.Marshal(map[string]string{"team_name": "to-delete"})
	result, err := deleteTool.Execute(context.Background(), deleteInput)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	if reg.Get("to-delete") != nil {
		t.Error("team should be removed after delete")
	}
}

func TestTeamDeleteTool_NonExistentErrors(t *testing.T) {
	reg := NewManagerRegistry()
	tool := TeamDeleteTool{Registry: reg}

	input, _ := json.Marshal(map[string]string{"team_name": "nope"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for deleting non-existent team")
	}
}

func TestTeamDeleteTool_MissingName(t *testing.T) {
	reg := NewManagerRegistry()
	tool := TeamDeleteTool{Registry: reg}

	input, _ := json.Marshal(map[string]string{})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing team_name")
	}
}

func TestTeamCreateTool_RegisteredWorkflow(t *testing.T) {
	reg := NewManagerRegistry()
	registered := map[string][]Phase{
		"custom-flow": {
			{Name: "step1", Tools: []string{"Read"}, Model: "claude-sonnet-4-6"},
		},
	}
	tool := TeamCreateTool{
		Registry:            reg,
		TaskDirBase:         t.TempDir(),
		RegisteredWorkflows: registered,
	}

	input, _ := json.Marshal(map[string]string{
		"team_name": "wf-team",
		"workflow":  "custom-flow",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	mgr := reg.Get("wf-team")
	if mgr == nil {
		t.Fatal("team should exist in registry after create")
	}
	if len(mgr.Config.Phases) != 1 {
		t.Fatalf("expected 1 phase from registered workflow, got %d", len(mgr.Config.Phases))
	}
	if mgr.Config.Phases[0].Name != "step1" {
		t.Errorf("expected phase 'step1', got %q", mgr.Config.Phases[0].Name)
	}
}

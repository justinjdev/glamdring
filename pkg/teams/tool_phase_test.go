package teams

import (
	"context"
	"encoding/json"
	"testing"
)

func setupPhaseTestTeam(t *testing.T) (*ManagerRegistry, *TeamManager) {
	t.Helper()
	reg := NewManagerRegistry()
	dir := t.TempDir()
	cfg := TeamConfig{
		Name: "phase-team",
		Phases: []Phase{
			{Name: "research", Tools: []string{"Read", "Grep"}, Model: "sonnet"},
			{Name: "plan", Tools: []string{"Read"}, Model: "sonnet"},
			{Name: "implement", Tools: []string{"Read", "Write"}, Model: "opus"},
		},
	}
	mgr, err := reg.Create(cfg, dir)
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	// Set phases for the agent.
	mgr.Phases.SetPhases("worker", cfg.Phases)
	return reg, mgr
}

func TestAdvancePhaseTool_AdvancesToNext(t *testing.T) {
	reg, _ := setupPhaseTestTeam(t)
	tool := AdvancePhaseTool{Registry: reg, AgentName: "worker"}

	input, _ := json.Marshal(map[string]string{"team_name": "phase-team"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	var out map[string]any
	json.Unmarshal([]byte(result.Output), &out)

	if out["phase_name"] != "plan" {
		t.Errorf("expected phase 'plan', got %v", out["phase_name"])
	}
	if out["model"] != "sonnet" {
		t.Errorf("expected model 'sonnet', got %v", out["model"])
	}
}

func TestAdvancePhaseTool_AdvancesToSpecific(t *testing.T) {
	reg, _ := setupPhaseTestTeam(t)
	tool := AdvancePhaseTool{Registry: reg, AgentName: "worker"}

	input, _ := json.Marshal(map[string]string{
		"team_name":  "phase-team",
		"phase_name": "implement",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	var out map[string]any
	json.Unmarshal([]byte(result.Output), &out)

	if out["phase_name"] != "implement" {
		t.Errorf("expected phase 'implement', got %v", out["phase_name"])
	}
	if out["model"] != "opus" {
		t.Errorf("expected model 'opus', got %v", out["model"])
	}

	// Verify tools in output.
	toolsList, ok := out["tools"].([]any)
	if !ok {
		t.Fatal("expected tools to be a list")
	}
	if len(toolsList) != 2 {
		t.Errorf("expected 2 tools, got %d", len(toolsList))
	}
}

func TestAdvancePhaseTool_AtLastPhaseErrors(t *testing.T) {
	reg, _ := setupPhaseTestTeam(t)
	tool := AdvancePhaseTool{Registry: reg, AgentName: "worker"}

	// Advance through all phases.
	input, _ := json.Marshal(map[string]string{"team_name": "phase-team"})
	tool.Execute(context.Background(), input) // research -> plan
	tool.Execute(context.Background(), input) // plan -> implement

	// Now at last phase; should error.
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error when advancing past last phase")
	}
}

func TestAdvancePhaseTool_TeamNotFound(t *testing.T) {
	reg := NewManagerRegistry()
	tool := AdvancePhaseTool{Registry: reg, AgentName: "worker"}

	input, _ := json.Marshal(map[string]string{"team_name": "nope"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for non-existent team")
	}
}

func TestAdvancePhaseTool_AgentNotInPhaseTracker(t *testing.T) {
	reg := NewManagerRegistry()
	dir := t.TempDir()
	reg.Create(TeamConfig{Name: "empty-team"}, dir)

	tool := AdvancePhaseTool{Registry: reg, AgentName: "unknown"}

	input, _ := json.Marshal(map[string]string{"team_name": "empty-team"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for agent not in phase tracker")
	}
}

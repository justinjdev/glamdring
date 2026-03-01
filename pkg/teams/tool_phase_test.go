package teams

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/justin/glamdring/pkg/tools"
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

func makeInput(teamName, summary string) json.RawMessage {
	input, _ := json.Marshal(map[string]string{"team_name": teamName, "summary": summary})
	return input
}

func TestAdvancePhaseTool_AdvancesToNext(t *testing.T) {
	reg, _ := setupPhaseTestTeam(t)
	tool := AdvancePhaseTool{Registry: reg, AgentName: "worker"}

	result, err := tool.Execute(context.Background(), makeInput("phase-team", "finished research"))
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

func TestAdvancePhaseTool_SequentialAdvance(t *testing.T) {
	reg, _ := setupPhaseTestTeam(t)
	tool := AdvancePhaseTool{Registry: reg, AgentName: "worker"}

	// Advance research -> plan.
	tool.Execute(context.Background(), makeInput("phase-team", "done research"))

	// Advance plan -> implement.
	result, err := tool.Execute(context.Background(), makeInput("phase-team", "done planning"))
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
	tool.Execute(context.Background(), makeInput("phase-team", "done 1"))
	tool.Execute(context.Background(), makeInput("phase-team", "done 2"))

	// Now at last phase; should error.
	result, err := tool.Execute(context.Background(), makeInput("phase-team", "done 3"))
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

	result, err := tool.Execute(context.Background(), makeInput("nope", "summary"))
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

	result, err := tool.Execute(context.Background(), makeInput("empty-team", "summary"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for agent not in phase tracker")
	}
}

func TestAdvancePhaseTool_EmptySummaryErrors(t *testing.T) {
	reg, _ := setupPhaseTestTeam(t)
	tool := AdvancePhaseTool{Registry: reg, AgentName: "worker"}

	result, err := tool.Execute(context.Background(), makeInput("phase-team", ""))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for empty summary")
	}
}

func TestAdvancePhaseTool_AutoGateWithSummary(t *testing.T) {
	reg := NewManagerRegistry()
	dir := t.TempDir()
	cfg := TeamConfig{
		Name: "auto-team",
		Phases: []Phase{
			{Name: "work", Tools: []string{"Read"}, Model: "sonnet"},
			{Name: "verify", Tools: []string{"Read"}, Model: "sonnet"},
		},
	}
	mgr, _ := reg.Create(cfg, dir)
	mgr.Phases.SetPhases("agent", cfg.Phases)

	tool := AdvancePhaseTool{Registry: reg, AgentName: "agent"}
	result, err := tool.Execute(context.Background(), makeInput("auto-team", "completed work"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	var out map[string]any
	json.Unmarshal([]byte(result.Output), &out)
	if out["phase_name"] != "verify" {
		t.Errorf("expected phase 'verify', got %v", out["phase_name"])
	}
}

// --- Leader gate tests ---

func setupLeaderGateTeam(t *testing.T) (*ManagerRegistry, *TeamManager, <-chan AgentMessage) {
	t.Helper()
	reg := NewManagerRegistry()
	dir := t.TempDir()
	cfg := TeamConfig{
		Name:   "leader-team",
		Leader: "lead",
		Phases: []Phase{
			{Name: "plan", Tools: []string{"Read"}, Model: "sonnet", Gate: "leader"},
			{Name: "implement", Tools: []string{"Write"}, Model: "opus"},
		},
	}
	mgr, err := reg.Create(cfg, dir)
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	mgr.Phases.SetPhases("worker", cfg.Phases)

	// Subscribe to lead's messages to capture the approval request.
	// Approval requests are priority messages.
	_, leadPriority, err := mgr.Messages.Subscribe("lead", 32)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	return reg, mgr, leadPriority
}

func TestAdvancePhaseTool_LeaderGateApproved(t *testing.T) {
	reg, _, leadCh := setupLeaderGateTeam(t)

	priorityCh := make(chan any, 8)
	tool := AdvancePhaseTool{
		Registry:   reg,
		AgentName:  "worker",
		PriorityCh: priorityCh,
	}

	// Run advance in background.
	type result struct {
		res tools.Result
		err error
	}
	done := make(chan result, 1)
	go func() {
		r, e := tool.Execute(context.Background(), makeInput("leader-team", "plan complete"))
		done <- result{r, e}
	}()

	// Wait for approval request to arrive at leader.
	select {
	case msg := <-leadCh:
		if msg.Kind != MessageKindApprovalRequest {
			t.Fatalf("expected approval_request, got %v", msg.Kind)
		}
		// Send approval response back via priority channel.
		approved := true
		priorityCh <- AgentMessage{
			Kind:      MessageKindApprovalResponse,
			From:      "lead",
			To:        "worker",
			RequestID: msg.RequestID,
			Approve:   &approved,
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for approval request")
	}

	// Wait for result.
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("Execute: %v", r.err)
		}
		if r.res.IsError {
			t.Fatalf("unexpected error: %s", r.res.Output)
		}
		var out map[string]any
		json.Unmarshal([]byte(r.res.Output), &out)
		if out["phase_name"] != "implement" {
			t.Errorf("expected 'implement', got %v", out["phase_name"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for advance result")
	}
}

func TestAdvancePhaseTool_LeaderGateRejected(t *testing.T) {
	reg, _, leadCh := setupLeaderGateTeam(t)

	priorityCh := make(chan any, 8)
	tool := AdvancePhaseTool{
		Registry:   reg,
		AgentName:  "worker",
		PriorityCh: priorityCh,
	}

	type result struct {
		res tools.Result
		err error
	}
	done := make(chan result, 1)
	go func() {
		r, e := tool.Execute(context.Background(), makeInput("leader-team", "plan needs review"))
		done <- result{r, e}
	}()

	select {
	case msg := <-leadCh:
		rejected := false
		priorityCh <- AgentMessage{
			Kind:      MessageKindApprovalResponse,
			From:      "lead",
			To:        "worker",
			RequestID: msg.RequestID,
			Approve:   &rejected,
			Content:   "plan incomplete",
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for approval request")
	}

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("Execute: %v", r.err)
		}
		if !r.res.IsError {
			t.Fatal("expected error for rejected approval")
		}
		if r.res.Output == "" {
			t.Fatal("expected rejection message in output")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for result")
	}
}

func TestAdvancePhaseTool_LeaderGateContextCancelled(t *testing.T) {
	reg, _, _ := setupLeaderGateTeam(t)

	priorityCh := make(chan any, 8)
	tool := AdvancePhaseTool{
		Registry:   reg,
		AgentName:  "worker",
		PriorityCh: priorityCh,
	}

	ctx, cancel := context.WithCancel(context.Background())

	type result struct {
		res tools.Result
		err error
	}
	done := make(chan result, 1)
	go func() {
		r, e := tool.Execute(ctx, makeInput("leader-team", "plan summary"))
		done <- result{r, e}
	}()

	// Wait briefly then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("Execute: %v", r.err)
		}
		if !r.res.IsError {
			t.Fatal("expected error for context cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for result")
	}
}

func TestAdvancePhaseTool_LeaderGateForceShutdown(t *testing.T) {
	reg, _, _ := setupLeaderGateTeam(t)

	priorityCh := make(chan any, 8)
	cancelled := false
	tool := AdvancePhaseTool{
		Registry:   reg,
		AgentName:  "worker",
		PriorityCh: priorityCh,
		CancelFunc: func() { cancelled = true },
	}

	type result struct {
		res tools.Result
		err error
	}
	done := make(chan result, 1)
	go func() {
		r, e := tool.Execute(context.Background(), makeInput("leader-team", "plan done"))
		done <- result{r, e}
	}()

	// Send force shutdown.
	time.Sleep(50 * time.Millisecond)
	priorityCh <- AgentMessage{
		Kind:  MessageKindShutdownRequest,
		From:  "lead",
		To:    "worker",
		Force: true,
	}

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("Execute: %v", r.err)
		}
		if !r.res.IsError {
			t.Fatal("expected error for force shutdown")
		}
		if !cancelled {
			t.Error("expected CancelFunc to be called")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for result")
	}
}

func TestAdvancePhaseTool_LeaderGateNoLeader(t *testing.T) {
	reg := NewManagerRegistry()
	dir := t.TempDir()
	cfg := TeamConfig{
		Name: "no-leader-team",
		Phases: []Phase{
			{Name: "plan", Tools: []string{"Read"}, Gate: "leader"},
			{Name: "work", Tools: []string{"Write"}},
		},
	}
	mgr, _ := reg.Create(cfg, dir)
	mgr.Phases.SetPhases("worker", cfg.Phases)

	tool := AdvancePhaseTool{
		Registry:  reg,
		AgentName: "worker",
	}

	result, err := tool.Execute(context.Background(), makeInput("no-leader-team", "done"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error when no leader is configured")
	}
}

// --- Condition gate tests ---

func TestAdvancePhaseTool_ConditionGatePasses(t *testing.T) {
	reg := NewManagerRegistry()
	dir := t.TempDir()
	cfg := TeamConfig{
		Name: "cond-team",
		Phases: []Phase{
			{Name: "check", Tools: []string{"Read"}, Gate: "condition", GateConfig: map[string]string{"command": "true"}},
			{Name: "work", Tools: []string{"Write"}},
		},
	}
	mgr, _ := reg.Create(cfg, dir)
	mgr.Phases.SetPhases("worker", cfg.Phases)

	tool := AdvancePhaseTool{Registry: reg, AgentName: "worker"}
	result, err := tool.Execute(context.Background(), makeInput("cond-team", "ready to advance"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Output)
	}

	var out map[string]any
	json.Unmarshal([]byte(result.Output), &out)
	if out["phase_name"] != "work" {
		t.Errorf("expected 'work', got %v", out["phase_name"])
	}
}

func TestAdvancePhaseTool_ConditionGateFails(t *testing.T) {
	reg := NewManagerRegistry()
	dir := t.TempDir()
	cfg := TeamConfig{
		Name: "cond-fail-team",
		Phases: []Phase{
			{Name: "check", Tools: []string{"Read"}, Gate: "condition", GateConfig: map[string]string{"command": "exit 1"}},
			{Name: "work", Tools: []string{"Write"}},
		},
	}
	mgr, _ := reg.Create(cfg, dir)
	mgr.Phases.SetPhases("worker", cfg.Phases)

	tool := AdvancePhaseTool{Registry: reg, AgentName: "worker"}
	result, err := tool.Execute(context.Background(), makeInput("cond-fail-team", "checking"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for failing condition")
	}
}

func TestAdvancePhaseTool_ConditionGateOutput(t *testing.T) {
	reg := NewManagerRegistry()
	dir := t.TempDir()
	cfg := TeamConfig{
		Name: "cond-output-team",
		Phases: []Phase{
			{Name: "check", Tools: []string{"Read"}, Gate: "condition", GateConfig: map[string]string{"command": "echo 'tests failed'; exit 1"}},
			{Name: "work", Tools: []string{"Write"}},
		},
	}
	mgr, _ := reg.Create(cfg, dir)
	mgr.Phases.SetPhases("worker", cfg.Phases)

	tool := AdvancePhaseTool{Registry: reg, AgentName: "worker"}
	result, _ := tool.Execute(context.Background(), makeInput("cond-output-team", "running checks"))
	if !result.IsError {
		t.Fatal("expected error")
	}
	if result.Output == "" {
		t.Fatal("expected output from failed command")
	}
}

func TestAdvancePhaseTool_ConditionGateContextCancel(t *testing.T) {
	reg := NewManagerRegistry()
	dir := t.TempDir()
	cfg := TeamConfig{
		Name: "cond-cancel-team",
		Phases: []Phase{
			{Name: "check", Tools: []string{"Read"}, Gate: "condition", GateConfig: map[string]string{"command": "sleep 60"}},
			{Name: "work", Tools: []string{"Write"}},
		},
	}
	mgr, _ := reg.Create(cfg, dir)
	mgr.Phases.SetPhases("worker", cfg.Phases)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	tool := AdvancePhaseTool{Registry: reg, AgentName: "worker"}
	result, _ := tool.Execute(ctx, makeInput("cond-cancel-team", "checking"))
	if !result.IsError {
		t.Fatal("expected error for cancelled context")
	}
}

func TestAdvancePhaseTool_ConditionGateMissingCommand(t *testing.T) {
	reg := NewManagerRegistry()
	dir := t.TempDir()
	cfg := TeamConfig{
		Name: "cond-nocmd-team",
		Phases: []Phase{
			{Name: "check", Tools: []string{"Read"}, Gate: "condition"},
			{Name: "work", Tools: []string{"Write"}},
		},
	}
	mgr, _ := reg.Create(cfg, dir)
	mgr.Phases.SetPhases("worker", cfg.Phases)

	tool := AdvancePhaseTool{Registry: reg, AgentName: "worker"}
	result, _ := tool.Execute(context.Background(), makeInput("cond-nocmd-team", "checking"))
	if !result.IsError {
		t.Fatal("expected error for missing command")
	}
}

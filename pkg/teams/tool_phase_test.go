package teams

import (
	"context"
	"encoding/json"
	"strings"
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

func TestAdvancePhaseTool_UnknownGateTypeErrors(t *testing.T) {
	reg := NewManagerRegistry()
	dir := t.TempDir()
	cfg := TeamConfig{
		Name: "bad-gate-team",
		Phases: []Phase{
			{Name: "work", Tools: []string{"Read"}, Gate: GateType("bogus")},
			{Name: "next", Tools: []string{"Write"}},
		},
	}
	mgr, _ := reg.Create(cfg, dir)
	mgr.Phases.SetPhases("worker", cfg.Phases)

	tool := AdvancePhaseTool{Registry: reg, AgentName: "worker"}
	result, err := tool.Execute(context.Background(), makeInput("bad-gate-team", "done"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for unknown gate type")
	}
	if !strings.Contains(result.Output, "unknown gate type") {
		t.Errorf("expected 'unknown gate type' in output, got: %s", result.Output)
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
			{Name: "plan", Tools: []string{"Read"}, Model: "sonnet", Gate: GateLeader},
			{Name: "implement", Tools: []string{"Write"}, Model: "opus"},
		},
	}
	mgr, err := reg.Create(cfg, dir)
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	mgr.Phases.SetPhases("worker", cfg.Phases)

	// Subscribe to lead's messages to capture the approval request.
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

	type result struct {
		res tools.Result
		err error
	}
	done := make(chan result, 1)
	go func() {
		r, e := tool.Execute(context.Background(), makeInput("leader-team", "plan complete"))
		done <- result{r, e}
	}()

	select {
	case msg := <-leadCh:
		if msg.Kind != MessageKindApprovalRequest {
			t.Fatalf("expected approval_request, got %v", msg.Kind)
		}
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
			{Name: "plan", Tools: []string{"Read"}, Gate: GateLeader},
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

func TestAdvancePhaseTool_LeaderGateConfigOverride(t *testing.T) {
	reg := NewManagerRegistry()
	dir := t.TempDir()
	cfg := TeamConfig{
		Name:   "override-team",
		Leader: "team-lead",
		Phases: []Phase{
			{Name: "plan", Tools: []string{"Read"}, Gate: GateLeader, GateConfig: map[string]string{"leader": "phase-lead"}},
			{Name: "work", Tools: []string{"Write"}},
		},
	}
	mgr, _ := reg.Create(cfg, dir)
	mgr.Phases.SetPhases("worker", cfg.Phases)

	// Subscribe to phase-lead (the override) to verify routing.
	_, phasePriority, err := mgr.Messages.Subscribe("phase-lead", 32)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

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
		r, e := tool.Execute(context.Background(), makeInput("override-team", "plan done"))
		done <- result{r, e}
	}()

	// The approval request should go to phase-lead, not team-lead.
	select {
	case msg := <-phasePriority:
		if msg.Kind != MessageKindApprovalRequest {
			t.Fatalf("expected approval_request, got %v", msg.Kind)
		}
		approved := true
		priorityCh <- AgentMessage{
			Kind:      MessageKindApprovalResponse,
			From:      "phase-lead",
			To:        "worker",
			RequestID: msg.RequestID,
			Approve:   &approved,
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for approval request at phase-lead")
	}

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("Execute: %v", r.err)
		}
		if r.res.IsError {
			t.Fatalf("unexpected error: %s", r.res.Output)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for result")
	}
}

func TestAdvancePhaseTool_LeaderGateFirstMemberFallback(t *testing.T) {
	reg := NewManagerRegistry()
	dir := t.TempDir()
	cfg := TeamConfig{
		Name: "fallback-team",
		// No Leader field set.
		Phases: []Phase{
			{Name: "plan", Tools: []string{"Read"}, Gate: GateLeader},
			{Name: "work", Tools: []string{"Write"}},
		},
	}
	mgr, _ := reg.Create(cfg, dir)
	mgr.Phases.SetPhases("worker", cfg.Phases)

	// Register members so firstMember returns "alice" (alphabetically first).
	mgr.Members.Add(Member{Name: "charlie", Status: MemberStatusActive})
	mgr.Members.Add(Member{Name: "alice", Status: MemberStatusActive})
	mgr.Members.Add(Member{Name: "worker", Status: MemberStatusActive})

	// Subscribe to alice's messages.
	_, alicePriority, err := mgr.Messages.Subscribe("alice", 32)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

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
		r, e := tool.Execute(context.Background(), makeInput("fallback-team", "plan done"))
		done <- result{r, e}
	}()

	// The approval request should go to alice (alphabetically first).
	select {
	case msg := <-alicePriority:
		if msg.Kind != MessageKindApprovalRequest {
			t.Fatalf("expected approval_request, got %v", msg.Kind)
		}
		approved := true
		priorityCh <- AgentMessage{
			Kind:      MessageKindApprovalResponse,
			From:      "alice",
			To:        "worker",
			RequestID: msg.RequestID,
			Approve:   &approved,
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for approval request at alice")
	}

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("Execute: %v", r.err)
		}
		if r.res.IsError {
			t.Fatalf("unexpected error: %s", r.res.Output)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for result")
	}
}

func TestAdvancePhaseTool_LeaderGateBufferedMessages(t *testing.T) {
	reg, mgr, leadCh := setupLeaderGateTeam(t)

	// Subscribe worker to capture re-sent messages (DMs go to regular channel).
	workerRegular, _, err := mgr.Messages.Subscribe("worker", 32)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

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
		r, e := tool.Execute(context.Background(), makeInput("leader-team", "plan complete"))
		done <- result{r, e}
	}()

	// Wait for approval request at lead.
	var requestID string
	select {
	case msg := <-leadCh:
		requestID = msg.RequestID
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for approval request")
	}

	// Send two unrelated DMs before the approval.
	priorityCh <- AgentMessage{Kind: MessageKindDM, From: "other", To: "worker", Content: "hello"}
	priorityCh <- AgentMessage{Kind: MessageKindDM, From: "other2", To: "worker", Content: "hi"}

	// Now send the approval.
	approved := true
	priorityCh <- AgentMessage{
		Kind:      MessageKindApprovalResponse,
		From:      "lead",
		To:        "worker",
		RequestID: requestID,
		Approve:   &approved,
	}

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
		buffered, _ := out["buffered_messages"].(float64)
		if buffered != 2 {
			t.Errorf("expected 2 buffered messages, got %v", buffered)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for result")
	}

	// Verify buffered messages were re-sent to the worker (DMs go to regular channel).
	received := 0
	timeout := time.After(1 * time.Second)
	for received < 2 {
		select {
		case msg := <-workerRegular:
			if msg.Kind != MessageKindDM {
				t.Errorf("expected DM, got %v", msg.Kind)
			}
			received++
		case <-timeout:
			t.Fatalf("expected 2 re-sent messages, got %d", received)
		}
	}
}

// --- Condition gate tests ---

func TestAdvancePhaseTool_ConditionGatePasses(t *testing.T) {
	reg := NewManagerRegistry()
	dir := t.TempDir()
	cfg := TeamConfig{
		Name: "cond-team",
		Phases: []Phase{
			{Name: "check", Tools: []string{"Read"}, Gate: GateCondition, GateConfig: map[string]string{"command": "true"}},
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
			{Name: "check", Tools: []string{"Read"}, Gate: GateCondition, GateConfig: map[string]string{"command": "exit 1"}},
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
			{Name: "check", Tools: []string{"Read"}, Gate: GateCondition, GateConfig: map[string]string{"command": "echo 'tests failed'; exit 1"}},
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
			{Name: "check", Tools: []string{"Read"}, Gate: GateCondition, GateConfig: map[string]string{"command": "sleep 60"}},
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
			{Name: "check", Tools: []string{"Read"}, Gate: GateCondition},
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

// --- GateType tests ---

func TestGateType_Valid(t *testing.T) {
	tests := []struct {
		gate  GateType
		valid bool
	}{
		{"", true},
		{GateAuto, true},
		{GateLeader, true},
		{GateCondition, true},
		{"bogus", false},
		{"leadr", false},
	}
	for _, tc := range tests {
		if got := tc.gate.Valid(); got != tc.valid {
			t.Errorf("GateType(%q).Valid() = %v, want %v", tc.gate, got, tc.valid)
		}
	}
}

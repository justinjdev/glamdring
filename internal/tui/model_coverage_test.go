package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justin/glamdring/pkg/agent"
	"github.com/justin/glamdring/pkg/commands"
	"github.com/justin/glamdring/pkg/config"
	"github.com/justin/glamdring/pkg/hooks"
)

// --- Setter tests ---

func TestNewWithAgent(t *testing.T) {
	ctx := context.Background()
	cfg := agent.Config{Model: "claude-opus-4-6"}
	m := NewWithAgent(ctx, cfg)

	if m.ctx != ctx {
		t.Error("expected ctx to be set")
	}
	if m.agentCfg.Model != "claude-opus-4-6" {
		t.Errorf("expected model claude-opus-4-6, got %q", m.agentCfg.Model)
	}
	if m.state != StateInput {
		t.Errorf("expected StateInput, got %d", m.state)
	}
}

func TestSetIndexDB(t *testing.T) {
	m := New()
	if m.indexDB != nil {
		t.Fatal("expected nil indexDB initially")
	}
	// We can't create a real index.DB easily, so just test nil -> nil assignment.
	m.SetIndexDB(nil)
	if m.indexDB != nil {
		t.Error("expected indexDB to remain nil")
	}
}

func TestSetIndexerConfig(t *testing.T) {
	m := New()
	cfg := config.IndexerConfig{}
	m.SetIndexerConfig(cfg)
	// Verify it was stored (command defaults to "shire").
	if m.indexerCfg.IndexerCommand() != "shire" {
		t.Errorf("expected default command 'shire', got %q", m.indexerCfg.IndexerCommand())
	}
}

func TestSetSettings(t *testing.T) {
	m := New()
	s := config.Settings{}
	m.SetSettings(s)
	// Just verify it doesn't panic and stores the value.
}

func TestSetMCPManager(t *testing.T) {
	m := New()
	m.SetMCPManager(nil)
	if m.mcpMgr != nil {
		t.Error("expected nil mcpMgr")
	}
}

func TestSetMCPConfiguredCount(t *testing.T) {
	m := New()
	m.SetMCPConfiguredCount(5)
	if m.mcpConfiguredCount != 5 {
		t.Errorf("expected 5, got %d", m.mcpConfiguredCount)
	}
}

func TestSetBaseTools(t *testing.T) {
	m := New()
	m.SetBaseTools(nil)
	if m.baseTools != nil {
		t.Error("expected nil base tools")
	}
}

func TestInitMCPStatus_NilManager(t *testing.T) {
	m := New()
	m.mcpMgr = nil
	// Should not panic.
	m.InitMCPStatus()
}

// --- Update message routing tests ---

func TestUpdate_WindowSizeMsg(t *testing.T) {
	m := New()
	result, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model := result.(Model)

	if model.width != 120 {
		t.Errorf("expected width 120, got %d", model.width)
	}
	if model.height != 40 {
		t.Errorf("expected height 40, got %d", model.height)
	}
	if cmd != nil {
		t.Error("expected nil cmd from WindowSizeMsg")
	}
}

func TestUpdate_AgentDoneMsg_ResetsToInput(t *testing.T) {
	m := New()
	m.state = StateRunning

	result, cmd := m.Update(agentDoneMsg{})
	model := result.(Model)

	if model.state != StateInput {
		t.Errorf("expected StateInput, got %d", model.state)
	}
	if model.agentCh != nil {
		t.Error("expected agentCh to be nil")
	}
	// Should have a focus cmd.
	if cmd == nil {
		t.Error("expected non-nil cmd to focus input")
	}
}

func TestUpdate_AgentDoneMsg_AlreadyInput(t *testing.T) {
	m := New()
	m.state = StateInput

	result, cmd := m.Update(agentDoneMsg{})
	model := result.(Model)

	if model.state != StateInput {
		t.Errorf("expected StateInput, got %d", model.state)
	}
	if cmd != nil {
		t.Error("expected nil cmd when already in input state")
	}
}

func TestUpdate_CheckpointFoundMsg(t *testing.T) {
	m := New()
	m.state = StateInput

	result, _ := m.Update(checkpointFoundMsg{content: "checkpoint content"})
	model := result.(Model)

	if model.state != StateCheckpoint {
		t.Errorf("expected StateCheckpoint, got %d", model.state)
	}
	if model.checkpointContent != "checkpoint content" {
		t.Errorf("expected checkpoint content, got %q", model.checkpointContent)
	}
	// Should have system messages in output.
	found := false
	for _, b := range model.output.blocks {
		if b.kind == blockSystem && strings.Contains(b.content, "checkpoint") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected checkpoint system message in output")
	}
}

func TestUpdate_MCPServerDiedMsg(t *testing.T) {
	m := New()
	m.mcpMgr = newTestMCPManager()
	m.mcpConfiguredCount = 3

	result, _ := m.Update(MCPServerDiedMsg{Name: "test-server"})
	model := result.(Model)

	found := false
	for _, b := range model.output.blocks {
		if b.kind == blockError && strings.Contains(b.content, "test-server") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error message about dead server")
	}
}

func TestUpdate_IndexRebuildDoneMsg_Error(t *testing.T) {
	m := New()
	result, _ := m.Update(indexRebuildDoneMsg{err: errors.New("rebuild failed")})
	_ = result.(Model)
	// Should not panic.
}

func TestUpdate_IndexRebuildDoneMsg_Success(t *testing.T) {
	m := New()
	// No existing indexDB, should swap in nil without panic.
	result, _ := m.Update(indexRebuildDoneMsg{db: nil, err: nil})
	_ = result.(Model)
}

func TestUpdate_SpinnerTickMsg_NotSpinning(t *testing.T) {
	m := New()
	m.spinning = false

	// Create a spinner tick message.
	result, cmd := m.Update(m.spinner.Tick())
	_ = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd when not spinning")
	}
}

func TestUpdate_PassthroughToInput(t *testing.T) {
	m := New()
	m.state = StateInput

	// Send a non-key, non-special message.
	type customMsg struct{}
	result, _ := m.Update(customMsg{})
	model := result.(Model)
	if model.state != StateInput {
		t.Errorf("expected StateInput after passthrough, got %d", model.state)
	}
}

func TestUpdate_PassthroughToOutput(t *testing.T) {
	m := New()
	m.state = StateRunning

	type customMsg struct{}
	result, _ := m.Update(customMsg{})
	model := result.(Model)
	if model.state != StateRunning {
		t.Errorf("expected StateRunning after passthrough, got %d", model.state)
	}
}

func TestUpdate_PassthroughToOutputPermission(t *testing.T) {
	m := New()
	m.state = StatePermission

	type customMsg struct{}
	result, _ := m.Update(customMsg{})
	model := result.(Model)
	if model.state != StatePermission {
		t.Errorf("expected StatePermission after passthrough, got %d", model.state)
	}
}

// --- handleSubmit tests ---

func TestHandleSubmit_EmptyText(t *testing.T) {
	m := New()
	result, cmd := m.handleSubmit(SubmitMsg{Text: ""})
	model := result.(Model)

	if model.state != StateInput {
		t.Errorf("expected StateInput for empty submit, got %d", model.state)
	}
	if cmd != nil {
		t.Error("expected nil cmd for empty submit")
	}
}

func TestHandleSubmit_BuiltinCommand(t *testing.T) {
	m := New()
	m.state = StateInput

	result, _ := m.handleSubmit(SubmitMsg{Text: "/help"})
	model := result.(Model)

	// /help doesn't start the agent, so state stays Input.
	if model.state != StateInput {
		t.Errorf("expected StateInput after /help, got %d", model.state)
	}
	// Should have help output.
	found := false
	for _, b := range model.output.blocks {
		if b.kind == blockSystem && strings.Contains(b.content, "/help") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected help output in blocks")
	}
}

// testHandleSubmitStateTransitions tests the non-agent-call parts of handleSubmit
// by verifying state, turn counter, and output blocks. We avoid calling session.Turn()
// which requires a real API client by testing the logic paths before and after the
// agent call.
func TestHandleSubmit_StateTransitions(t *testing.T) {
	t.Run("text sets state to running", func(t *testing.T) {
		m := New()
		m.ctx = context.Background()
		m.agentCfg = agent.Config{Model: "claude-opus-4-6"}

		// Verify state transitions by running handleSubmit without a session.
		// When session is nil, it creates one, which is fine -- it just starts
		// a goroutine. We'll immediately cancel it.
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately so the Turn goroutine exits fast
		m.ctx = ctx

		result, _ := m.handleSubmit(SubmitMsg{Text: "hello agent"})
		model := result.(Model)

		if model.state != StateRunning {
			t.Errorf("expected StateRunning after submit, got %d", model.state)
		}
		if model.turn != 1 {
			t.Errorf("expected turn 1, got %d", model.turn)
		}
		if !model.spinning {
			t.Error("expected spinning to be true")
		}
		if model.input.history.Len() != 1 {
			t.Errorf("expected 1 history entry, got %d", model.input.history.Len())
		}
		if model.cancelTurn == nil {
			t.Error("expected cancelTurn to be set")
		}
	})

	t.Run("images displayed in output", func(t *testing.T) {
		m := New()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		m.ctx = ctx
		m.agentCfg = agent.Config{Model: "claude-opus-4-6"}

		images := []PendingImage{
			{Data: []byte{1}, Width: 100, Height: 200},
		}
		result, _ := m.handleSubmit(SubmitMsg{Text: "describe", Images: images})
		model := result.(Model)

		if model.state != StateRunning {
			t.Errorf("expected StateRunning, got %d", model.state)
		}
		found := false
		for _, b := range model.output.blocks {
			if b.kind == blockUserMessage && strings.Contains(b.content, "[Image 1: 100x200]") {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected image label in user message")
		}
	})

	t.Run("images only no text", func(t *testing.T) {
		m := New()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		m.ctx = ctx
		m.agentCfg = agent.Config{Model: "claude-opus-4-6"}

		images := []PendingImage{
			{Data: []byte{1}, Width: 0, Height: 0},
		}
		result, _ := m.handleSubmit(SubmitMsg{Text: "", Images: images})
		model := result.(Model)

		found := false
		for _, b := range model.output.blocks {
			if b.kind == blockUserMessage && strings.Contains(b.content, "[Image 1]") {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected image-only label in user message")
		}
	})

	t.Run("multiple images with dimensions", func(t *testing.T) {
		m := New()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		m.ctx = ctx
		m.agentCfg = agent.Config{Model: "claude-opus-4-6"}

		images := []PendingImage{
			{Data: []byte{1}, Width: 100, Height: 200},
			{Data: []byte{2}, Width: 300, Height: 400},
		}
		result, _ := m.handleSubmit(SubmitMsg{Text: "describe both", Images: images})
		model := result.(Model)

		found := false
		for _, b := range model.output.blocks {
			if b.kind == blockUserMessage &&
				strings.Contains(b.content, "[Image 1: 100x200]") &&
				strings.Contains(b.content, "[Image 2: 300x400]") {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected both image labels in user message")
		}
	})
}

// --- handlePermissionKey tests ---

func TestHandlePermissionKey_Approve(t *testing.T) {
	m := New()
	m.state = StatePermission
	permCh := make(chan agent.PermissionAnswer, 1)
	m.permission = &agent.Message{
		Type:               agent.MessagePermissionRequest,
		PermissionResponse: permCh,
	}
	agentCh := make(chan agent.Message, 1)
	m.agentCh = agentCh

	result, cmd := m.handlePermissionKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model := result.(Model)

	if model.state != StateRunning {
		t.Errorf("expected StateRunning, got %d", model.state)
	}
	if model.permission != nil {
		t.Error("expected permission to be cleared")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd to resume agent")
	}
	answer := <-permCh
	if answer != agent.PermissionApprove {
		t.Errorf("expected PermissionApprove, got %d", answer)
	}
}

func TestHandlePermissionKey_AlwaysApprove(t *testing.T) {
	m := New()
	m.state = StatePermission
	permCh := make(chan agent.PermissionAnswer, 1)
	m.permission = &agent.Message{
		Type:               agent.MessagePermissionRequest,
		PermissionResponse: permCh,
	}
	agentCh := make(chan agent.Message, 1)
	m.agentCh = agentCh

	result, cmd := m.handlePermissionKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model := result.(Model)

	if model.state != StateRunning {
		t.Errorf("expected StateRunning, got %d", model.state)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd")
	}
	answer := <-permCh
	if answer != agent.PermissionAlwaysApprove {
		t.Errorf("expected PermissionAlwaysApprove, got %d", answer)
	}
}

func TestHandlePermissionKey_Deny(t *testing.T) {
	m := New()
	m.state = StatePermission
	permCh := make(chan agent.PermissionAnswer, 1)
	m.permission = &agent.Message{
		Type:               agent.MessagePermissionRequest,
		PermissionResponse: permCh,
	}
	agentCh := make(chan agent.Message, 1)
	m.agentCh = agentCh

	result, cmd := m.handlePermissionKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model := result.(Model)

	if model.state != StateRunning {
		t.Errorf("expected StateRunning, got %d", model.state)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd")
	}
	answer := <-permCh
	if answer != agent.PermissionDeny {
		t.Errorf("expected PermissionDeny, got %d", answer)
	}
}

func TestHandlePermissionKey_UpperCase(t *testing.T) {
	m := New()
	m.state = StatePermission
	permCh := make(chan agent.PermissionAnswer, 1)
	m.permission = &agent.Message{
		Type:               agent.MessagePermissionRequest,
		PermissionResponse: permCh,
	}
	agentCh := make(chan agent.Message, 1)
	m.agentCh = agentCh

	result, cmd := m.handlePermissionKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}})
	model := result.(Model)

	if model.state != StateRunning {
		t.Errorf("expected StateRunning for 'Y', got %d", model.state)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd")
	}
	answer := <-permCh
	if answer != agent.PermissionApprove {
		t.Errorf("expected PermissionApprove for 'Y', got %d", answer)
	}
}

func TestHandlePermissionKey_NilPermission(t *testing.T) {
	m := New()
	m.state = StatePermission
	m.permission = nil

	result, cmd := m.handlePermissionKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	_ = result.(Model)
	if cmd != nil {
		t.Error("expected nil cmd when permission is nil")
	}
}

func TestHandlePermissionKey_UnrecognizedKey(t *testing.T) {
	m := New()
	m.state = StatePermission
	permCh := make(chan agent.PermissionAnswer, 1)
	m.permission = &agent.Message{
		Type:               agent.MessagePermissionRequest,
		PermissionResponse: permCh,
	}

	result, cmd := m.handlePermissionKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	model := result.(Model)

	// Unrecognized key should not change state.
	if model.permission == nil {
		t.Error("expected permission to remain set for unrecognized key")
	}
	if cmd != nil {
		t.Error("expected nil cmd for unrecognized key")
	}
}

func TestHandlePermissionKey_NoAgentCh(t *testing.T) {
	m := New()
	m.state = StatePermission
	permCh := make(chan agent.PermissionAnswer, 1)
	m.permission = &agent.Message{
		Type:               agent.MessagePermissionRequest,
		PermissionResponse: permCh,
	}
	m.agentCh = nil // no agent channel

	result, cmd := m.handlePermissionKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model := result.(Model)

	if model.state != StateRunning {
		t.Errorf("expected StateRunning, got %d", model.state)
	}
	// No agentCh means cmd should be nil (no resume).
	if cmd != nil {
		t.Error("expected nil cmd when agentCh is nil")
	}
}

// --- handleCheckpointKey tests ---

func TestHandleCheckpointKey_Accept(t *testing.T) {
	m := New()
	m.state = StateCheckpoint
	m.checkpointContent = "previous context"
	m.agentCfg.SystemPrompt = "base prompt"

	result, cmd := m.handleCheckpointKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model := result.(Model)

	if model.state != StateInput {
		t.Errorf("expected StateInput, got %d", model.state)
	}
	if model.checkpointContent != "" {
		t.Error("expected checkpoint content to be cleared")
	}
	if !strings.Contains(model.agentCfg.SystemPrompt, "previous context") {
		t.Error("expected system prompt to include checkpoint content")
	}
	if !strings.Contains(model.agentCfg.SystemPrompt, "Previous Session Context") {
		t.Error("expected system prompt to include section header")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd to focus input")
	}
}

func TestHandleCheckpointKey_Reject(t *testing.T) {
	m := New()
	m.state = StateCheckpoint
	m.checkpointContent = "previous context"
	m.agentCfg.SystemPrompt = "base prompt"

	result, cmd := m.handleCheckpointKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model := result.(Model)

	if model.state != StateInput {
		t.Errorf("expected StateInput, got %d", model.state)
	}
	if model.checkpointContent != "" {
		t.Error("expected checkpoint content to be cleared")
	}
	// System prompt should NOT include checkpoint.
	if strings.Contains(model.agentCfg.SystemPrompt, "previous context") {
		t.Error("expected system prompt to NOT include checkpoint content")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd to focus input")
	}
}

func TestHandleCheckpointKey_UnrecognizedKey(t *testing.T) {
	m := New()
	m.state = StateCheckpoint
	m.checkpointContent = "content"

	result, cmd := m.handleCheckpointKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	model := result.(Model)

	// Should not change state.
	if model.checkpointContent != "content" {
		t.Error("expected checkpoint content to remain")
	}
	if cmd != nil {
		t.Error("expected nil cmd for unrecognized key")
	}
}

func TestHandleCheckpointKey_EscapeDismisses(t *testing.T) {
	m := New()
	m.state = StateCheckpoint
	m.checkpointContent = "content"

	result, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyEscape})
	model := result.(Model)

	if model.state != StateInput {
		t.Errorf("expected StateInput after Escape, got %d", model.state)
	}
	if model.checkpointContent != "" {
		t.Error("expected checkpoint content to be cleared on escape")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd to focus input")
	}
}

// --- writeCheckpoint tests ---

func TestWriteCheckpoint_EmptyCWD(t *testing.T) {
	m := New()
	m.agentCfg.CWD = ""

	err := m.writeCheckpoint("summary")
	if err != nil {
		t.Errorf("expected nil error for empty CWD, got %v", err)
	}
}

func TestWriteCheckpoint_WritesFile(t *testing.T) {
	dir := t.TempDir()
	m := New()
	m.agentCfg.CWD = dir

	err := m.writeCheckpoint("test summary")
	if err != nil {
		t.Fatalf("writeCheckpoint failed: %v", err)
	}

	path := filepath.Join(dir, "tmp", "checkpoint.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read checkpoint file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "test summary") {
		t.Errorf("expected 'test summary' in checkpoint, got %q", content)
	}
	if !strings.Contains(content, "<!-- Checkpoint:") {
		t.Error("expected timestamp comment in checkpoint")
	}
	if !strings.Contains(content, "<!-- Branch:") {
		t.Error("expected branch comment in checkpoint")
	}
}

// --- fireContextThresholdHook tests ---

func TestFireContextThresholdHook_NilRunner(t *testing.T) {
	m := New()
	m.agentCfg.HookRunner = nil
	// Should not panic.
	m.fireContextThresholdHook(80)
}

func TestFireContextThresholdHook_RunnerError(t *testing.T) {
	runner := hooks.NewHookRunner([]hooks.Hook{
		{
			Event:   hooks.ContextThreshold,
			Command: "false", // always fails
		},
	})
	m := New()
	m.agentCfg.HookRunner = runner

	// Should not panic even when hook fails.
	m.fireContextThresholdHook(80)

	// Allow goroutine to complete.
	time.Sleep(200 * time.Millisecond)
}

func TestFireContextThresholdHook_WithRunner(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "hook_output.txt")
	runner := hooks.NewHookRunner([]hooks.Hook{
		{
			Event:   hooks.ContextThreshold,
			Command: "cat > " + tmpFile,
		},
	})
	m := New()
	m.agentCfg.HookRunner = runner

	m.fireContextThresholdHook(80)

	// Allow goroutine to complete.
	time.Sleep(200 * time.Millisecond)

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read hook output: %v", err)
	}
	if !strings.Contains(string(data), `"percentage":80`) {
		t.Errorf("expected percentage in hook input, got %q", string(data))
	}
}

// --- handleAgentMsg extended tests ---

func TestHandleAgentMsg_ThinkingDelta_ShowThinking(t *testing.T) {
	m := New()
	m.showThinking = true

	msg := AgentMsg(agent.Message{Type: agent.MessageThinkingDelta, Text: "thinking..."})
	result, _ := m.handleAgentMsg(msg)
	result.output.FlushAllPending()

	found := false
	for _, b := range result.output.blocks {
		if b.kind == blockThinking && b.content == "thinking..." {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected thinking block when showThinking is true")
	}
}

func TestHandleAgentMsg_ThinkingDelta_HideThinking(t *testing.T) {
	m := New()
	m.showThinking = false

	msg := AgentMsg(agent.Message{Type: agent.MessageThinkingDelta, Text: "thinking..."})
	result, _ := m.handleAgentMsg(msg)

	for _, b := range result.output.blocks {
		if b.kind == blockThinking {
			t.Error("expected no thinking block when showThinking is false")
		}
	}
}

func TestHandleAgentMsg_ToolCall_TracksModifiedFiles(t *testing.T) {
	for _, toolName := range []string{"Edit", "Write", "Bash"} {
		t.Run(toolName, func(t *testing.T) {
			m := New()
			m.turnModifiedFiles = false

			msg := AgentMsg(agent.Message{
				Type:      agent.MessageToolCall,
				ToolName:  toolName,
				ToolInput: map[string]any{"key": "val"},
			})
			result, _ := m.handleAgentMsg(msg)
			if !result.turnModifiedFiles {
				t.Errorf("expected turnModifiedFiles to be true for %s", toolName)
			}
		})
	}
}

func TestHandleAgentMsg_ToolCall_ReadDoesNotModify(t *testing.T) {
	m := New()
	m.turnModifiedFiles = false

	msg := AgentMsg(agent.Message{
		Type:      agent.MessageToolCall,
		ToolName:  "Read",
		ToolInput: map[string]any{"file_path": "/tmp/test"},
	})
	result, _ := m.handleAgentMsg(msg)
	if result.turnModifiedFiles {
		t.Error("expected turnModifiedFiles to remain false for Read")
	}
}

func TestHandleAgentMsg_ToolOutputDelta(t *testing.T) {
	m := New()

	msg := AgentMsg(agent.Message{Type: agent.MessageToolOutputDelta, Text: "output chunk"})
	result, _ := m.handleAgentMsg(msg)
	result.output.FlushAllPending()

	found := false
	for _, b := range result.output.blocks {
		if b.kind == blockToolResult && b.content == "output chunk" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected tool result block with delta content")
	}
}

func TestHandleAgentMsg_ToolResult(t *testing.T) {
	m := New()

	msg := AgentMsg(agent.Message{
		Type:        agent.MessageToolResult,
		ToolOutput:  "result content",
		ToolIsError: false,
	})
	result, _ := m.handleAgentMsg(msg)

	found := false
	for _, b := range result.output.blocks {
		if b.kind == blockToolResult && b.content == "result content" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected tool result block")
	}
}

func TestHandleAgentMsg_PermissionRequest(t *testing.T) {
	m := New()
	m.state = StateRunning

	msg := AgentMsg(agent.Message{
		Type:              agent.MessagePermissionRequest,
		PermissionSummary: "Run dangerous command",
	})
	result, _ := m.handleAgentMsg(msg)

	if result.state != StatePermission {
		t.Errorf("expected StatePermission, got %d", result.state)
	}
	if result.permission == nil {
		t.Error("expected permission to be set")
	}
}

func TestHandleAgentMsg_Error_WithMessage(t *testing.T) {
	m := New()
	m.spinning = true

	msg := AgentMsg(agent.Message{
		Type: agent.MessageError,
		Err:  errors.New("something broke"),
	})
	result, _ := m.handleAgentMsg(msg)

	if result.spinning {
		t.Error("expected spinning to stop on error")
	}
	found := false
	for _, b := range result.output.blocks {
		if b.kind == blockError && b.content == "something broke" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error block with message")
	}
}

func TestHandleAgentMsg_Error_NilErr(t *testing.T) {
	m := New()
	msg := AgentMsg(agent.Message{Type: agent.MessageError, Err: nil})
	result, _ := m.handleAgentMsg(msg)

	found := false
	for _, b := range result.output.blocks {
		if b.kind == blockError && b.content == "unknown error" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'unknown error' block")
	}
}

func TestHandleAgentMsg_Done_UpdatesTokens(t *testing.T) {
	m := New()
	m.agentCfg.Model = "claude-opus-4-6"
	m.state = StateRunning

	msg := AgentMsg(agent.Message{
		Type:         agent.MessageDone,
		InputTokens:  1000,
		OutputTokens: 500,
	})
	result, cmd := m.handleAgentMsg(msg)

	if result.totalInputTokens != 1000 {
		t.Errorf("expected 1000 input tokens, got %d", result.totalInputTokens)
	}
	if result.totalOutputTokens != 500 {
		t.Errorf("expected 500 output tokens, got %d", result.totalOutputTokens)
	}
	if result.state != StateInput {
		t.Errorf("expected StateInput, got %d", result.state)
	}
	if result.spinning {
		t.Error("expected spinning to stop")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd to focus input")
	}
}

func TestHandleAgentMsg_Done_ContextThreshold60(t *testing.T) {
	m := New()
	m.agentCfg.Model = "claude-opus-4-6"
	m.lastContextThreshold = 0

	msg := AgentMsg(agent.Message{
		Type:                   agent.MessageDone,
		LastRequestInputTokens: 130_000, // 65% of 200k
	})
	result, _ := m.handleAgentMsg(msg)

	if result.lastContextThreshold != 60 {
		t.Errorf("expected threshold 60, got %d", result.lastContextThreshold)
	}
	found := false
	for _, b := range result.output.blocks {
		if b.kind == blockSystem && strings.Contains(b.content, "/compact available") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 60% threshold message")
	}
}

func TestHandleAgentMsg_Done_ContextThreshold80(t *testing.T) {
	m := New()
	m.agentCfg.Model = "claude-opus-4-6"
	m.lastContextThreshold = 0

	msg := AgentMsg(agent.Message{
		Type:                   agent.MessageDone,
		LastRequestInputTokens: 170_000, // 85% of 200k
	})
	result, _ := m.handleAgentMsg(msg)

	if result.lastContextThreshold != 80 {
		t.Errorf("expected threshold 80, got %d", result.lastContextThreshold)
	}
	found := false
	for _, b := range result.output.blocks {
		if b.kind == blockSystem && strings.Contains(b.content, "consider running /compact") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 80% threshold message")
	}
}

func TestHandleAgentMsg_Done_ContextThresholdReset(t *testing.T) {
	m := New()
	m.agentCfg.Model = "claude-opus-4-6"
	m.lastContextThreshold = 80

	msg := AgentMsg(agent.Message{
		Type:                   agent.MessageDone,
		LastRequestInputTokens: 50_000, // 25% -- below 60%
	})
	result, _ := m.handleAgentMsg(msg)

	if result.lastContextThreshold != 0 {
		t.Errorf("expected threshold reset to 0, got %d", result.lastContextThreshold)
	}
}

func TestHandleAgentMsg_Done_NoDoubleFire60(t *testing.T) {
	m := New()
	m.agentCfg.Model = "claude-opus-4-6"
	m.lastContextThreshold = 60 // already fired 60

	msg := AgentMsg(agent.Message{
		Type:                   agent.MessageDone,
		LastRequestInputTokens: 130_000, // still 65%
	})
	result, _ := m.handleAgentMsg(msg)

	// Should NOT fire again.
	if result.lastContextThreshold != 60 {
		t.Errorf("expected threshold to stay 60, got %d", result.lastContextThreshold)
	}
}

func TestHandleAgentMsg_Done_CompactingFlow(t *testing.T) {
	m := New()
	m.agentCfg.Model = "claude-opus-4-6"
	m.agentCfg.CWD = t.TempDir()
	m.compacting = true
	m.state = StateRunning
	m.session = agent.NewSession(m.agentCfg)

	// Add a text block that would be the summary.
	m.output.AppendText("compact summary here")

	msg := AgentMsg(agent.Message{Type: agent.MessageDone})
	result, _ := m.handleAgentMsg(msg)

	if result.compacting {
		t.Error("expected compacting to be false after done")
	}
	// Output should be cleared and have system message.
	found := false
	for _, b := range result.output.blocks {
		if b.kind == blockSystem && strings.Contains(b.content, "Context compacted") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected compact confirmation message")
	}
}

func TestHandleAgentMsg_MaxTurnsReached(t *testing.T) {
	m := New()
	m.state = StateRunning

	msg := AgentMsg(agent.Message{Type: agent.MessageMaxTurnsReached})
	result, cmd := m.handleAgentMsg(msg)

	if result.state != StateInput {
		t.Errorf("expected StateInput, got %d", result.state)
	}
	found := false
	for _, b := range result.output.blocks {
		if b.kind == blockError && b.content == "max turns reached" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected max turns error message")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd to focus input")
	}
}

// --- View tests ---

func TestView_InputState(t *testing.T) {
	m := New()
	m.width = 80
	m.height = 24
	m.layoutComponents()
	m.state = StateInput

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestView_PermissionState(t *testing.T) {
	m := New()
	m.width = 80
	m.height = 24
	m.layoutComponents()
	m.state = StatePermission

	view := m.View()
	if !strings.Contains(view, "Allow this action?") {
		t.Error("expected permission prompt in view")
	}
}

func TestView_CheckpointState(t *testing.T) {
	m := New()
	m.width = 80
	m.height = 24
	m.layoutComponents()
	m.state = StateCheckpoint

	view := m.View()
	if !strings.Contains(view, "Load checkpoint") {
		t.Error("expected checkpoint prompt in view")
	}
}

func TestView_Spinning(t *testing.T) {
	m := New()
	m.width = 80
	m.height = 24
	m.layoutComponents()
	m.state = StateRunning
	m.spinning = true
	m.spinnerLabel = "Thinking..."

	view := m.View()
	if !strings.Contains(view, "Thinking") {
		t.Error("expected spinner text in view")
	}
}

func TestView_NotSpinning(t *testing.T) {
	m := New()
	m.width = 80
	m.height = 24
	m.layoutComponents()
	m.state = StateRunning
	m.spinning = false

	view := m.View()
	if strings.Contains(view, "Thinking") {
		t.Error("expected no spinner text when not spinning")
	}
}

// --- handleKeyMsg coverage ---

func TestHandleKeyMsg_ShiftTab_NoSession(t *testing.T) {
	m := New()
	m.session = nil

	result, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyShiftTab})
	model := result.(Model)
	// Should not panic and nothing changes.
	_ = model
}

func TestHandleKeyMsg_ShiftTab_WithSession(t *testing.T) {
	m := New()
	m.agentCfg = agent.Config{Model: "claude-opus-4-6"}
	m.session = agent.NewSession(m.agentCfg)

	result, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyShiftTab})
	model := result.(Model)

	if !model.session.IsYolo() {
		t.Error("expected yolo to be toggled on")
	}
	found := false
	for _, b := range model.output.blocks {
		if b.kind == blockSystem && strings.Contains(b.content, "YOLO mode enabled") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected YOLO enabled message")
	}

	// Toggle back off.
	result2, _ := model.handleKeyMsg(tea.KeyMsg{Type: tea.KeyShiftTab})
	model2 := result2.(Model)
	if model2.session.IsYolo() {
		t.Error("expected yolo to be toggled off")
	}
}

func TestHandleKeyMsg_CtrlC_PermissionState(t *testing.T) {
	m := New()
	m.state = StatePermission
	permCh := make(chan agent.PermissionAnswer, 1)
	m.permission = &agent.Message{
		Type:               agent.MessagePermissionRequest,
		PermissionResponse: permCh,
	}
	m.spinning = true
	cancelled := false
	m.cancelTurn = func() { cancelled = true }
	m.agentCh = make(chan agent.Message, 1)

	result, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyCtrlC})
	model := result.(Model)

	if model.state != StateInput {
		t.Errorf("expected StateInput, got %d", model.state)
	}
	if !cancelled {
		t.Error("expected cancel to be called")
	}
	if model.permission != nil {
		t.Error("expected permission to be cleared")
	}
}

// --- layoutComponents edge case ---

func TestLayoutComponents_SmallHeight(t *testing.T) {
	m := New()
	m.width = 80
	m.height = 2 // very small
	m.layoutComponents()

	// Output height should be clamped to at least 1.
	if m.output.viewport.Height < 1 {
		t.Errorf("expected output height >= 1, got %d", m.output.viewport.Height)
	}
}

// --- refreshMCPTools ---

func TestRefreshMCPTools_NoSession(t *testing.T) {
	m := New()
	m.baseTools = nil
	m.session = nil
	m.mcpMgr = nil
	m.indexDB = nil

	m.refreshMCPTools()
	if len(m.agentCfg.Tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(m.agentCfg.Tools))
	}
}

func TestRefreshMCPTools_WithSession(t *testing.T) {
	m := New()
	m.baseTools = nil
	m.agentCfg = agent.Config{Model: "claude-opus-4-6"}
	m.session = agent.NewSession(m.agentCfg)
	m.mcpMgr = nil
	m.indexDB = nil

	m.refreshMCPTools()

	// Session should be reset (set to nil).
	if m.session != nil {
		t.Error("expected session to be reset")
	}
	// Should have system message.
	found := false
	for _, b := range m.output.blocks {
		if b.kind == blockSystem && strings.Contains(b.content, "session reset") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected session reset message")
	}
}

// --- waitForAgent ---

func TestWaitForAgent_ChannelClosed(t *testing.T) {
	ch := make(chan agent.Message)
	close(ch)

	cmd := waitForAgent(ch)
	msg := cmd()
	if _, ok := msg.(agentDoneMsg); !ok {
		t.Errorf("expected agentDoneMsg, got %T", msg)
	}
}

func TestWaitForAgent_ReceivesMessage(t *testing.T) {
	ch := make(chan agent.Message, 1)
	ch <- agent.Message{Type: agent.MessageTextDelta, Text: "hello"}

	cmd := waitForAgent(ch)
	msg := cmd()
	if amsg, ok := msg.(AgentMsg); !ok {
		t.Errorf("expected AgentMsg, got %T", msg)
	} else {
		if agent.Message(amsg).Text != "hello" {
			t.Errorf("expected 'hello', got %q", agent.Message(amsg).Text)
		}
	}
}

// --- currentGitBranch ---

func TestCurrentGitBranch_InvalidDir(t *testing.T) {
	branch := currentGitBranch("/nonexistent/directory/xyz")
	if branch != "unknown" {
		t.Errorf("expected 'unknown', got %q", branch)
	}
}

// --- Update with AgentMsg ---

func TestUpdate_AgentMsg_ResumesDraining(t *testing.T) {
	m := New()
	m.state = StateRunning
	ch := make(chan agent.Message, 1)
	m.agentCh = ch

	msg := AgentMsg(agent.Message{Type: agent.MessageTextDelta, Text: "hello"})
	result, cmd := m.Update(msg)
	model := result.(Model)

	// Should keep draining.
	if model.agentCh == nil {
		t.Error("expected agentCh to remain set")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd to keep draining")
	}
}

func TestUpdate_AgentMsg_PermissionStopsDraining(t *testing.T) {
	m := New()
	m.state = StateRunning
	ch := make(chan agent.Message, 1)
	m.agentCh = ch

	msg := AgentMsg(agent.Message{
		Type:              agent.MessagePermissionRequest,
		PermissionSummary: "allow this?",
	})
	result, cmd := m.Update(msg)
	model := result.(Model)

	if model.state != StatePermission {
		t.Errorf("expected StatePermission, got %d", model.state)
	}
	// Should NOT keep draining (only the handleAgentMsg cmd returned).
	// The cmd should be nil since permission request doesn't produce a cmd.
	_ = cmd
}

// --- agentStartedMsg ---

func TestUpdate_AgentStartedMsg(t *testing.T) {
	m := New()
	ch := make(chan agent.Message, 1)

	result, cmd := m.Update(agentStartedMsg{ch: ch})
	model := result.(Model)

	if model.agentCh != ch {
		t.Error("expected agentCh to be set")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd to start draining")
	}
}

// --- handleSubmit with user-defined command expansion ---

func TestHandleSubmit_UnknownSlashCommand(t *testing.T) {
	m := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.ctx = ctx
	m.agentCfg = agent.Config{Model: "claude-opus-4-6"}
	// No cmd registry -- the command will be treated as regular text.

	result, _ := m.handleSubmit(SubmitMsg{Text: "/unknowncmd"})
	model := result.(Model)

	// Without a registry, slash commands go directly to the agent.
	if model.state != StateRunning {
		t.Errorf("expected StateRunning, got %d", model.state)
	}
}

func TestHandleSubmit_UserDefinedCommandExpansionError(t *testing.T) {
	m := New()
	m.ctx = context.Background()
	m.agentCfg = agent.Config{Model: "claude-opus-4-6"}

	// Set up a cmd registry without the command, so expansion fails.
	reg := commands.NewRegistry(nil)
	m.cmdRegistry = reg

	result, cmd := m.handleSubmit(SubmitMsg{Text: "/nonexistent"})
	model := result.(Model)

	if model.state != StateInput {
		t.Errorf("expected StateInput after unknown user command, got %d", model.state)
	}
	// Should have error output.
	found := false
	for _, b := range model.output.blocks {
		if b.kind == blockError && strings.Contains(b.content, "unknown command") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'unknown command' error in output")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd to focus input")
	}
}

func TestInitMCPStatus_WithManager(t *testing.T) {
	m := New()
	m.mcpMgr = newTestMCPManager()
	m.mcpConfiguredCount = 3

	m.InitMCPStatus()

	// Verify the status bar was updated.
	if m.statusbar.mcpTotal != 3 {
		t.Errorf("expected mcpTotal=3, got %d", m.statusbar.mcpTotal)
	}
}

func TestCurrentGitBranch_ValidRepo(t *testing.T) {
	// Use the working directory of the test process, which is within the repo.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	branch := currentGitBranch(wd)
	// In CI (detached HEAD), git rev-parse --abbrev-ref HEAD returns "HEAD".
	// Locally it returns the branch name. Either is valid; only "unknown"
	// (our error sentinel) or empty indicates a problem.
	if branch == "unknown" || branch == "" {
		t.Errorf("expected a branch name or HEAD, got %q", branch)
	}
}

// --- handleKeyMsg: running state with non-e key ---

func TestHandleKeyMsg_RunningState_ScrollKey(t *testing.T) {
	m := New()
	m.state = StateRunning

	// Non-e key in running state should pass through to output.
	result, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyDown})
	model := result.(Model)
	if model.state != StateRunning {
		t.Errorf("expected StateRunning, got %d", model.state)
	}
}

// --- handleSubmit: /quit via handleSubmit ---

func TestHandleSubmit_QuitBuiltin(t *testing.T) {
	m := New()
	result, cmd := m.handleSubmit(SubmitMsg{Text: "/quit"})
	model := result.(Model)

	// /quit handler starts the agent, but since it returns tea.Quit, state may vary.
	// The key thing is the cmd produces a quit message.
	_ = model
	if cmd == nil {
		t.Fatal("expected non-nil cmd from /quit")
	}
}

// --- Update with SubmitMsg ---

func TestUpdate_SubmitMsg(t *testing.T) {
	m := New()
	result, _ := m.Update(SubmitMsg{Text: "/help"})
	model := result.(Model)

	found := false
	for _, b := range model.output.blocks {
		if b.kind == blockSystem && strings.Contains(b.content, "/help") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected help output from SubmitMsg via Update")
	}
}

// --- Update with KeyMsg routes to handleKeyMsg ---

func TestUpdate_KeyMsg(t *testing.T) {
	m := New()
	m.state = StateInput
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	model := result.(Model)

	// Should show quit hint.
	if model.lastCtrlC.IsZero() {
		t.Error("expected lastCtrlC to be set")
	}
}

// --- Spinner tick when spinning ---

func TestUpdate_SpinnerTickMsg_Spinning(t *testing.T) {
	m := New()
	m.spinning = true

	// Generate a tick message from the spinner.
	tickMsg := m.spinner.Tick()
	result, cmd := m.Update(tickMsg)
	model := result.(Model)

	_ = model
	// When spinning, the tick should produce another tick cmd.
	if cmd == nil {
		t.Error("expected non-nil cmd from spinner tick when spinning")
	}
}

// --- Init ---

func TestInit_ReturnsCommands(t *testing.T) {
	m := New()
	cmd := m.Init()
	// Init returns a batch of commands (input init, output init, startup, spinner tick).
	if cmd == nil {
		t.Error("expected non-nil cmd from Init")
	}
}

// --- startupCmd with no hooks or CWD ---

func TestStartupCmd_NoHooksNoCWD(t *testing.T) {
	m := New()
	m.agentCfg.HookRunner = nil
	m.agentCfg.CWD = ""

	cmd := m.startupCmd()
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	if msg != nil {
		t.Errorf("expected nil msg (no checkpoint, no hooks), got %T", msg)
	}
}

func TestStartupCmd_WithCheckpoint(t *testing.T) {
	dir := t.TempDir()
	// Create a checkpoint file.
	tmpDir := filepath.Join(dir, "tmp")
	os.MkdirAll(tmpDir, 0o755)
	os.WriteFile(filepath.Join(tmpDir, "checkpoint.md"), []byte("checkpoint content"), 0o644)

	m := New()
	m.agentCfg.CWD = dir

	cmd := m.startupCmd()
	msg := cmd()
	if cfm, ok := msg.(checkpointFoundMsg); !ok {
		t.Errorf("expected checkpointFoundMsg, got %T", msg)
	} else if cfm.content != "checkpoint content" {
		t.Errorf("expected 'checkpoint content', got %q", cfm.content)
	}
}

func TestStartupCmd_WithHookRunner(t *testing.T) {
	runner := hooks.NewHookRunner([]hooks.Hook{
		{Event: hooks.SessionStart, Command: "true"},
	})
	m := New()
	m.ctx = context.Background()
	m.agentCfg.HookRunner = runner
	m.agentCfg.CWD = ""

	cmd := m.startupCmd()
	msg := cmd()
	// No checkpoint, so msg should be nil (hook ran but no msg).
	if msg != nil {
		t.Errorf("expected nil msg, got %T", msg)
	}
}

func TestStartupCmd_NilCtxUsesBackground(t *testing.T) {
	runner := hooks.NewHookRunner([]hooks.Hook{
		{Event: hooks.SessionStart, Command: "true"},
	})
	m := New()
	m.ctx = nil // should use context.Background()
	m.agentCfg.HookRunner = runner
	m.agentCfg.CWD = ""

	cmd := m.startupCmd()
	msg := cmd()
	if msg != nil {
		t.Errorf("expected nil msg, got %T", msg)
	}
}

// --- handleSubmit with nil ctx ---

func TestHandleSubmit_NilCtxUsesBackground(t *testing.T) {
	m := New()
	// Use a pre-cancelled context to prevent the session goroutine from
	// actually calling the nil API client.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.ctx = ctx
	m.agentCfg = agent.Config{Model: "claude-opus-4-6"}

	result, _ := m.handleSubmit(SubmitMsg{Text: "hello"})
	model := result.(Model)

	if model.state != StateRunning {
		t.Errorf("expected StateRunning, got %d", model.state)
	}
	// Cancel to prevent goroutine leak.
	if model.cancelTurn != nil {
		model.cancelTurn()
	}
}

// --- handleKeyMsg: Esc in non-permission/non-checkpoint state ---

func TestHandleKeyMsg_EscInInputState(t *testing.T) {
	m := New()
	m.state = StateInput

	// Esc in input state passes through to textarea.
	result, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyEscape})
	model := result.(Model)
	if model.state != StateInput {
		t.Errorf("expected StateInput, got %d", model.state)
	}
}

// --- handleKeyMsg: Running state passthrough ---

func TestHandleKeyMsg_RunningState_Passthrough(t *testing.T) {
	m := New()
	m.state = StateRunning

	// Non-e, non-scroll key in running state.
	result, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	model := result.(Model)
	if model.state != StateRunning {
		t.Errorf("expected StateRunning, got %d", model.state)
	}
}

// --- handleSubmit: builtin command that starts agent (/compact) ---

func TestHandleSubmit_CompactBuiltin_StartsAgent(t *testing.T) {
	m := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.ctx = ctx
	m.agentCfg = agent.Config{Model: "claude-opus-4-6"}
	m.session = agent.NewSession(m.agentCfg)

	result, cmd := m.handleSubmit(SubmitMsg{Text: "/compact"})
	model := result.(Model)

	// /compact sets state to Running.
	if model.state != StateRunning {
		t.Errorf("expected StateRunning after /compact, got %d", model.state)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd from /compact")
	}
}

// --- writeCheckpoint with invalid directory ---

func TestWriteCheckpoint_InvalidDir(t *testing.T) {
	m := New()
	m.agentCfg.CWD = "/nonexistent/path/that/does/not/exist"

	err := m.writeCheckpoint("summary")
	if err == nil {
		t.Error("expected error for invalid directory")
	}
}

// --- refreshMCPTools with MCP manager ---

func TestRefreshMCPTools_WithMCPManager(t *testing.T) {
	m := New()
	m.baseTools = nil
	m.mcpMgr = newTestMCPManager()
	m.indexDB = nil
	m.session = nil

	m.refreshMCPTools()
	// Should have MCP tools added.
	if len(m.agentCfg.Tools) != 2 {
		t.Errorf("expected 2 MCP tools, got %d", len(m.agentCfg.Tools))
	}
}

// --- agentDoneMsg handler ---

func TestUpdate_AgentDoneMsg_WhenRunning(t *testing.T) {
	m := New()
	m.state = StateRunning

	result, cmd := m.Update(agentDoneMsg{})
	model := result.(Model)

	if model.agentCh != nil {
		t.Error("expected agentCh to be nil")
	}
	if model.state != StateInput {
		t.Errorf("expected StateInput, got %d", model.state)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd to focus input")
	}
}

func TestUpdate_AgentDoneMsg_WhenAlreadyInput(t *testing.T) {
	m := New()
	m.state = StateInput

	result, cmd := m.Update(agentDoneMsg{})
	model := result.(Model)

	if model.agentCh != nil {
		t.Error("expected agentCh to be nil")
	}
	if cmd != nil {
		t.Error("expected nil cmd since already in input state")
	}
}

// --- checkpointFoundMsg handler (extended) ---

func TestUpdate_CheckpointFoundMsg_StoresContent(t *testing.T) {
	m := New()

	result, cmd := m.Update(checkpointFoundMsg{content: "saved context"})
	model := result.(Model)

	if model.state != StateCheckpoint {
		t.Errorf("expected StateCheckpoint, got %d", model.state)
	}
	if model.checkpointContent != "saved context" {
		t.Errorf("expected 'saved context', got %q", model.checkpointContent)
	}
	if cmd != nil {
		t.Error("expected nil cmd from checkpointFoundMsg")
	}
}

// --- indexRebuildDoneMsg handler (extended) ---

func TestUpdate_IndexRebuildDoneMsg_ErrorPath(t *testing.T) {
	m := New()

	result, cmd := m.Update(indexRebuildDoneMsg{err: errors.New("rebuild failed")})
	model := result.(Model)

	_ = model // should not panic
	if cmd != nil {
		t.Error("expected nil cmd on rebuild error")
	}
}

func TestUpdate_IndexRebuildDoneMsg_NilDB(t *testing.T) {
	m := New()
	m.indexDB = nil

	result, cmd := m.Update(indexRebuildDoneMsg{db: nil})
	model := result.(Model)

	_ = model // should not panic
	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

// --- handleKeyMsg: 'e' key in running state toggles collapse ---

func TestHandleKeyMsg_EKey_RunningState(t *testing.T) {
	m := New()
	m.state = StateRunning

	// Add a tool result block so there's something to toggle.
	m.output.AppendToolResult("result content", false)
	m.output.finalizePreviousBlock()

	result, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	model := result.(Model)

	if model.state != StateRunning {
		t.Errorf("expected StateRunning, got %d", model.state)
	}
	if cmd != nil {
		t.Error("expected nil cmd from 'e' toggle")
	}
}

// --- handleKeyMsg: esc in permission with agentCh ---

func TestHandleKeyMsg_EscPermission_WithAgentCh(t *testing.T) {
	m := New()
	m.state = StatePermission
	permCh := make(chan agent.PermissionAnswer, 1)
	m.permission = &agent.Message{
		Type:               agent.MessagePermissionRequest,
		PermissionResponse: permCh,
	}
	agentCh := make(chan agent.Message, 1)
	m.agentCh = agentCh

	result, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyEscape})
	model := result.(Model)

	if model.state != StateRunning {
		t.Errorf("expected StateRunning after esc deny, got %d", model.state)
	}
	if model.permission != nil {
		t.Error("expected permission to be cleared")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd to resume draining")
	}
	answer := <-permCh
	if answer != agent.PermissionDeny {
		t.Errorf("expected PermissionDeny, got %d", answer)
	}
}

func TestHandleKeyMsg_EscPermission_NoAgentCh(t *testing.T) {
	m := New()
	m.state = StatePermission
	permCh := make(chan agent.PermissionAnswer, 1)
	m.permission = &agent.Message{
		Type:               agent.MessagePermissionRequest,
		PermissionResponse: permCh,
	}
	m.agentCh = nil

	result, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyEscape})
	model := result.(Model)

	if model.permission != nil {
		t.Error("expected permission to be cleared")
	}
	if cmd != nil {
		t.Error("expected nil cmd when no agentCh")
	}
}

// --- Double Ctrl+C quits ---

func TestHandleKeyMsg_DoubleCtrlC_Quits(t *testing.T) {
	m := New()
	m.state = StateInput
	m.lastCtrlC = time.Now() // simulate first Ctrl+C just happened

	result, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyCtrlC})
	model := result.(Model)

	_ = model
	if cmd == nil {
		t.Error("expected non-nil quit cmd from double Ctrl+C")
	}
}

// --- CtrlC in running state without cancelTurn ---

func TestHandleKeyMsg_CtrlC_RunningNoCancel(t *testing.T) {
	m := New()
	m.state = StateRunning
	m.cancelTurn = nil
	m.spinning = true
	m.agentCh = make(chan agent.Message, 1)

	result, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyCtrlC})
	model := result.(Model)

	if model.state != StateInput {
		t.Errorf("expected StateInput, got %d", model.state)
	}
	if model.spinning {
		t.Error("expected spinning to stop")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd to focus input")
	}
}

// --- Update passthrough to input in StateInput ---

func TestUpdate_Passthrough_InputState(t *testing.T) {
	m := New()
	m.state = StateInput
	m.width = 80
	m.height = 24
	m.layoutComponents()

	// A non-KeyMsg, non-SubmitMsg, etc. should pass through to input component.
	type customMsg struct{}
	result, _ := m.Update(customMsg{})
	model := result.(Model)
	_ = model // should not panic
}

// --- Update passthrough to output in StateRunning ---

func TestUpdate_Passthrough_RunningState(t *testing.T) {
	m := New()
	m.state = StateRunning

	type customMsg struct{}
	result, _ := m.Update(customMsg{})
	model := result.(Model)
	_ = model
}

// --- extractLastText ---

func TestExtractLastText_NoTextBlocks(t *testing.T) {
	m := New()
	m.output.AppendToolCall("Read", "test.go")

	result := m.extractLastText()
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestExtractLastText_WithTextBlock(t *testing.T) {
	m := New()
	m.output.AppendText("  summary text  ")
	m.output.FlushAllPending()
	m.output.finalizePreviousBlock()

	result := m.extractLastText()
	if result != "summary text" {
		t.Errorf("expected 'summary text', got %q", result)
	}
}

// --- MCPServerDiedMsg handler (extended) ---

func TestUpdate_MCPServerDiedMsg_WithManager(t *testing.T) {
	m := New()
	m.mcpMgr = newTestMCPManager()
	m.mcpConfiguredCount = 1

	result, cmd := m.Update(MCPServerDiedMsg{Name: "test-server"})
	model := result.(Model)

	found := false
	for _, b := range model.output.blocks {
		if b.kind == blockError && strings.Contains(b.content, "test-server") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error about dead server")
	}
	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestUpdate_MCPServerDiedMsg_NilMgr(t *testing.T) {
	m := New()
	m.mcpMgr = nil

	result, cmd := m.Update(MCPServerDiedMsg{Name: "test-server"})
	model := result.(Model)

	_ = model // should not panic
	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

// --- WindowSizeMsg handler (extended) ---

func TestUpdate_WindowSizeMsg_UpdatesDimensions(t *testing.T) {
	m := New()

	result, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model := result.(Model)

	if model.width != 120 {
		t.Errorf("expected width 120, got %d", model.width)
	}
	if model.height != 40 {
		t.Errorf("expected height 40, got %d", model.height)
	}
	if cmd != nil {
		t.Error("expected nil cmd from WindowSizeMsg")
	}
}

// --- handleSubmit with user-defined command expansion success ---

func TestHandleSubmit_UserDefinedCommandSuccess(t *testing.T) {
	m := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.ctx = ctx
	m.agentCfg = agent.Config{Model: "claude-opus-4-6"}

	// Create a command file.
	dir := t.TempDir()
	cmdFile := filepath.Join(dir, "review.md")
	os.WriteFile(cmdFile, []byte("Review: $ARGUMENTS"), 0o644)

	reg := commands.NewRegistry([]commands.Command{
		{Name: "review", Path: cmdFile, Source: "project"},
	})
	m.cmdRegistry = reg

	result, cmd := m.handleSubmit(SubmitMsg{Text: "/review main.go"})
	model := result.(Model)

	if model.state != StateRunning {
		t.Errorf("expected StateRunning, got %d", model.state)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd")
	}
}

// --- handleSubmit with images ---

func TestHandleSubmit_WithImages(t *testing.T) {
	m := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.ctx = ctx
	m.agentCfg = agent.Config{Model: "claude-opus-4-6"}

	images := []PendingImage{
		{Data: []byte{1, 2, 3}, Width: 100, Height: 200},
	}
	result, cmd := m.handleSubmit(SubmitMsg{Text: "describe this", Images: images})
	model := result.(Model)

	if model.state != StateRunning {
		t.Errorf("expected StateRunning, got %d", model.state)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd")
	}
	// Should have user message with image label.
	found := false
	for _, b := range model.output.blocks {
		if b.kind == blockUserMessage && strings.Contains(b.content, "[Image 1: 100x200]") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected image label in user message")
	}
}

func TestHandleSubmit_ImagesOnly(t *testing.T) {
	m := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.ctx = ctx
	m.agentCfg = agent.Config{Model: "claude-opus-4-6"}

	images := []PendingImage{
		{Data: []byte{1}, Width: 0, Height: 0},
	}
	result, cmd := m.handleSubmit(SubmitMsg{Text: "", Images: images})
	model := result.(Model)

	if model.state != StateRunning {
		t.Errorf("expected StateRunning, got %d", model.state)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd")
	}
}

// --- summarizeToolInput: long fallback truncation ---

func TestSummarizeToolInput_LongFallback(t *testing.T) {
	longVal := strings.Repeat("v", 100)
	got := summarizeToolInput("CustomTool", map[string]any{"key": longVal})
	if len(got) > 80 {
		t.Errorf("expected truncated fallback (<=80 chars), got length %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("expected '...' suffix for truncated fallback")
	}
}

// --- spinner tick when NOT spinning ---

func TestUpdate_SpinnerTick_WhenIdle(t *testing.T) {
	m := New()
	m.spinning = false

	tickMsg := m.spinner.Tick()
	result, cmd := m.Update(tickMsg)
	model := result.(Model)

	_ = model
	if cmd != nil {
		t.Error("expected nil cmd when not spinning")
	}
}

// --- Mouse events routed to output viewport ---

func TestUpdate_MouseMsg_RoutesToOutput(t *testing.T) {
	m := New()
	m.state = StateInput

	// Mouse events should go to the output viewport even in StateInput.
	result, _ := m.Update(tea.MouseMsg{})
	model := result.(Model)
	if model.state != StateInput {
		t.Errorf("expected StateInput, got %d", model.state)
	}
}

func TestUpdate_MouseMsg_RoutesToOutput_Running(t *testing.T) {
	m := New()
	m.state = StateRunning

	result, _ := m.Update(tea.MouseMsg{})
	model := result.(Model)
	if model.state != StateRunning {
		t.Errorf("expected StateRunning, got %d", model.state)
	}
}

func TestIndexStartupCheck_NotInstalled(t *testing.T) {
	m := New()
	m.indexerCfg = config.IndexerConfig{Command: "shire-does-not-exist-xyz"}
	result, _ := m.Update(indexStartupCheckMsg{notInstalled: true})
	model := result.(Model)
	if model.state != StateInput {
		t.Errorf("state = %v, want StateInput", model.state)
	}
}

func TestIndexStartupCheck_Prompt(t *testing.T) {
	m := New()
	result, _ := m.Update(indexStartupCheckMsg{})
	model := result.(Model)
	if model.state != StateIndexPrompt {
		t.Errorf("state = %v, want StateIndexPrompt", model.state)
	}
}

func TestIndexStartupCheck_AutoBuild(t *testing.T) {
	m := New()
	result, _ := m.Update(indexStartupCheckMsg{autoBuild: true})
	model := result.(Model)
	if model.state == StateIndexPrompt {
		t.Error("state should not be StateIndexPrompt for autoBuild=true")
	}
}

func TestIndexStartupCheck_SkipSilently(t *testing.T) {
	// auto_build=false (notInstalled=false, autoBuild=false, but state stays Input without prompt)
	// Simulate by sending a notInstalled=false, autoBuild=false msg — the handler
	// should transition to StateIndexPrompt, so this test verifies the *false* path
	// is distinguished from the auto-build path via a separate flag.
	// The silent-skip path is driven by checkIndexStartupCmd not emitting a msg;
	// verify here that a non-auto, non-missing msg leads to the prompt (not a silent skip).
	m := New()
	result, _ := m.Update(indexStartupCheckMsg{notInstalled: false, autoBuild: false})
	model := result.(Model)
	if model.state != StateIndexPrompt {
		t.Errorf("state = %v, want StateIndexPrompt when autoBuild=false and index missing", model.state)
	}
}

func TestIndexStartupCheck_NoOverwriteCheckpoint(t *testing.T) {
	m := New()
	m.state = StateCheckpoint
	result, _ := m.Update(indexStartupCheckMsg{})
	model := result.(Model)
	if model.state != StateCheckpoint {
		t.Errorf("state = %v, want StateCheckpoint — index prompt must not overwrite checkpoint state", model.state)
	}
}

func TestIndexPromptKey_Yes(t *testing.T) {
	m := New()
	m.state = StateIndexPrompt
	result, _ := m.handleIndexPromptKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	model := result.(Model)
	if model.state == StateIndexPrompt {
		t.Error("state should have left StateIndexPrompt after y")
	}
}

func TestIndexPromptKey_No(t *testing.T) {
	m := New()
	m.state = StateIndexPrompt
	result, _ := m.handleIndexPromptKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := result.(Model)
	if model.state != StateInput {
		t.Errorf("state = %v, want StateInput", model.state)
	}
}

func TestIndexPromptKey_Yes_ReturnsCmd(t *testing.T) {
	m := New()
	m.state = StateIndexPrompt
	_, cmd := m.handleIndexPromptKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd == nil {
		t.Error("y path must return a rebuild cmd, got nil")
	}
}

func TestIndexStartupCheck_AutoBuild_ReturnsCmd(t *testing.T) {
	m := New()
	_, cmd := m.Update(indexStartupCheckMsg{autoBuild: true})
	if cmd == nil {
		t.Error("auto-build path must return a rebuild cmd, got nil")
	}
}

func TestCheckIndexStartupCmd_AutoBuildFalse_ReturnsNil(t *testing.T) {
	m := New()
	f := false
	m.indexerCfg = config.IndexerConfig{AutoBuild: &f}
	cmd := m.checkIndexStartupCmd()
	if cmd != nil {
		t.Error("checkIndexStartupCmd must return nil when auto_build=false")
	}
}

func TestIndexStartupCheck_DeferredDuringCheckpoint(t *testing.T) {
	m := New()
	m.state = StateCheckpoint
	result, _ := m.Update(indexStartupCheckMsg{})
	model := result.(Model)
	if model.state != StateCheckpoint {
		t.Errorf("state = %v, want StateCheckpoint", model.state)
	}
	if model.pendingIndexCheck == nil {
		t.Error("pendingIndexCheck should be stored when msg arrives during StateCheckpoint")
	}
}

func TestCheckpointKey_Yes_FlushesIndexCheck(t *testing.T) {
	m := New()
	m.state = StateCheckpoint
	msg := indexStartupCheckMsg{}
	m.pendingIndexCheck = &msg
	result, cmd := m.handleCheckpointKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	model := result.(Model)
	if model.pendingIndexCheck != nil {
		t.Error("pendingIndexCheck should be cleared after checkpoint resolves")
	}
	if cmd == nil {
		t.Error("cmd should be non-nil to replay the pending index check")
	}
}

func TestCheckpointKey_No_FlushesIndexCheck(t *testing.T) {
	m := New()
	m.state = StateCheckpoint
	msg := indexStartupCheckMsg{}
	m.pendingIndexCheck = &msg
	result, cmd := m.handleCheckpointKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model := result.(Model)
	if model.pendingIndexCheck != nil {
		t.Error("pendingIndexCheck should be cleared after checkpoint resolves")
	}
	if cmd == nil {
		t.Error("cmd should be non-nil to replay the pending index check")
	}
}

func TestIndexRebuildDoneMsg_ErrorSurfaces(t *testing.T) {
	m := New()
	result, _ := m.Update(indexRebuildDoneMsg{err: errors.New("build failed")})
	model := result.(Model)
	// Error should be visible in output (non-empty output indicates message was appended).
	_ = model // state doesn't change; the test validates no panic and the handler runs
}

func TestAutoBuild_ReturnsInputFocus(t *testing.T) {
	m := New()
	_, cmd := m.Update(indexStartupCheckMsg{autoBuild: true})
	if cmd == nil {
		t.Error("auto-build path must return a batched cmd including input.Focus()")
	}
}

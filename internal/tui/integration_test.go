package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justin/glamdring/pkg/agent"
	"github.com/justin/glamdring/pkg/tools"
)

func TestIntegration_SubmitAndReceiveResponse(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("Hello from the agent!", "end_turn"),
	})

	th.Submit("hi there")

	// Should be back in input state.
	if th.State() != StateInput {
		t.Errorf("expected StateInput, got %d", th.State())
	}

	// Output should contain the agent's response.
	if !th.OutputContains("Hello from the agent!") {
		t.Errorf("expected output to contain agent response, blocks: %+v", th.Model.output.blocks)
	}

	// Should have the user message displayed.
	th.RequireViewContains("hi there")
}

func TestIntegration_MultiTurnConversation(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("First response", "end_turn"),
		buildSSEResponse("Second response", "end_turn"),
	})

	th.Submit("first question")
	if !th.OutputContains("First response") {
		t.Error("expected first response in output")
	}

	th.Submit("second question")
	if !th.OutputContains("Second response") {
		t.Error("expected second response in output")
	}

	// Both user messages should be visible.
	th.RequireViewContains("first question")
	th.RequireViewContains("second question")
}

func TestIntegration_CtrlCInterruptsAgent(t *testing.T) {
	// Use a server that blocks (never sends a response) to simulate
	// a long-running agent turn.
	blockCh := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Block until test closes the channel.
		<-blockCh
	}))
	t.Cleanup(func() {
		close(blockCh)
		srv.Close()
	})

	ctx := context.Background()
	cfg := agent.Config{
		Model:    "test-model",
		Creds:    mockCreds{},
		Endpoint: srv.URL,
		Yolo:     true,
	}
	m := NewWithAgent(ctx, cfg)

	th := &TestHarness{t: t, Model: m, server: srv}
	th.Model.width = 80
	th.Model.height = 24
	th.Model.layoutComponents()
	cmd := th.Model.Init()
	if cmd != nil {
		th.pending = append(th.pending, cmd)
	}

	// Submit triggers the agent -- but don't DrainAgent since the server blocks.
	th.update(SubmitMsg{Text: "do something slow"})
	if th.State() != StateRunning {
		t.Fatalf("expected StateRunning after submit, got %d", th.State())
	}

	// Ctrl+C should interrupt.
	th.SendKey("ctrl+c")
	if th.State() != StateInput {
		t.Errorf("expected StateInput after Ctrl+C, got %d", th.State())
	}

	// Should show interrupted message.
	if !th.OutputContains("(interrupted)") {
		t.Error("expected '(interrupted)' in output")
	}
}

func TestIntegration_DoubleCtrlCQuits(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("ok", "end_turn"),
	})

	// First Ctrl+C -- should show hint.
	th.SendKey("ctrl+c")
	if !th.OutputContains("press Ctrl+C again to quit") {
		t.Error("expected quit hint after first Ctrl+C")
	}

	// Second Ctrl+C -- should produce quit command.
	result, cmd := th.Model.Update(parseKeyMsg("ctrl+c"))
	th.Model = result.(Model)
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestIntegration_BuiltinHelpCommand(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("unused", "end_turn"),
	})

	// /help is a built-in command that should not hit the agent.
	th.update(SubmitMsg{Text: "/help"})

	// Should still be in input state (not running).
	if th.State() != StateInput {
		t.Errorf("expected StateInput after /help, got %d", th.State())
	}

	// Should show help content in output.
	if !th.OutputContains("/help") || !th.OutputContains("/clear") {
		t.Error("expected help content listing commands")
	}
}

func TestIntegration_BuiltinClearCommand(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("some output", "end_turn"),
	})

	// First get some output.
	th.Submit("generate output")
	if th.BlockCount() == 0 {
		t.Fatal("expected blocks after submit")
	}

	// /clear should wipe the output.
	th.update(SubmitMsg{Text: "/clear"})
	if th.BlockCount() != 0 {
		t.Errorf("expected 0 blocks after /clear, got %d", th.BlockCount())
	}
}

func TestIntegration_APIError(t *testing.T) {
	// Server returns a non-retryable error (400) to avoid retry delays.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"type":"error","error":{"type":"invalid_request_error","message":"bad request"}}`)
	}))
	t.Cleanup(srv.Close)

	ctx := context.Background()
	cfg := agent.Config{
		Model:    "test-model",
		Creds:    mockCreds{},
		Endpoint: srv.URL,
		Yolo:     true,
	}
	m := NewWithAgent(ctx, cfg)

	th := &TestHarness{t: t, Model: m, server: srv}
	th.Model.width = 80
	th.Model.height = 24
	th.Model.layoutComponents()
	cmd := th.Model.Init()
	if cmd != nil {
		th.pending = append(th.pending, cmd)
	}

	th.Submit("trigger error")

	// Should be back in input state.
	if th.State() != StateInput {
		t.Errorf("expected StateInput after error, got %d", th.State())
	}

	// Should show an error in output.
	if th.BlocksOfKind(blockError) == 0 {
		t.Error("expected error block in output")
	}
}

func TestIntegration_ToolCallWithAutoApprove(t *testing.T) {
	// First response: tool call; second response: text after tool result.
	th := NewTestHarness(t, []string{
		buildToolUseResponse("Read", "tu_1", `{"file_path":"/tmp/test.txt"}`),
		buildSSEResponse("I read the file!", "end_turn"),
	}, WithSize(120, 40))

	// Register a mock Read tool.
	th.Model.agentCfg.Tools = []tools.Tool{&mockTool{
		name:   "Read",
		result: tools.Result{Output: "file contents here"},
	}}

	th.Submit("read a file")

	if th.State() != StateInput {
		t.Errorf("expected StateInput, got %d", th.State())
	}

	// Should show the tool call in output.
	if th.BlocksOfKind(blockToolCall) == 0 {
		t.Error("expected tool call block")
	}

	// Should show the agent's response after tool execution.
	if !th.OutputContains("I read the file!") {
		t.Error("expected agent response after tool call")
	}
}

func TestIntegration_PermissionPromptApprove(t *testing.T) {
	// First response: tool call requiring permission; second: text.
	th := NewTestHarness(t, []string{
		buildToolUseResponse("Bash", "tu_1", `{"command":"echo hello"}`),
		buildSSEResponse("Command executed!", "end_turn"),
	})

	// Disable yolo so Bash requires permission.
	th.Model.agentCfg.Yolo = false

	// Register a mock Bash tool.
	th.Model.agentCfg.Tools = []tools.Tool{&mockTool{
		name:   "Bash",
		result: tools.Result{Output: "hello"},
	}}

	// Submit and let it run until permission prompt.
	th.update(SubmitMsg{Text: "run a command"})
	th.DrainAgent()

	if th.State() != StatePermission {
		t.Fatalf("expected StatePermission, got %d", th.State())
	}

	// Press 'y' to approve.
	th.SendKey("y")

	// Drain remaining agent messages.
	th.DrainAgent()

	if th.State() != StateInput {
		t.Errorf("expected StateInput after approval, got %d", th.State())
	}

	if !th.OutputContains("Command executed!") {
		t.Error("expected agent response after permission approval")
	}
}

func TestIntegration_PermissionPromptDeny(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildToolUseResponse("Bash", "tu_1", `{"command":"rm -rf /"}`),
		buildSSEResponse("Ok, I won't do that.", "end_turn"),
	})

	th.Model.agentCfg.Yolo = false
	th.Model.agentCfg.Tools = []tools.Tool{&mockTool{
		name:   "Bash",
		result: tools.Result{Output: ""},
	}}

	th.update(SubmitMsg{Text: "delete everything"})
	th.DrainAgent()

	if th.State() != StatePermission {
		t.Fatalf("expected StatePermission, got %d", th.State())
	}

	// Press 'n' to deny.
	th.SendKey("n")
	th.DrainAgent()

	if th.State() != StateInput {
		t.Errorf("expected StateInput after denial, got %d", th.State())
	}

	// Should have continued (agent responds to denied tool).
	if !th.OutputContains("Ok, I won't do that.") {
		t.Error("expected agent response after permission denial")
	}
}

func TestIntegration_ViewRendersWithoutPanic(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("Test rendering", "end_turn"),
	})

	// View should render without panic at every stage.
	_ = th.View()

	th.Submit("test")
	view := th.View()
	if view == "" {
		t.Error("expected non-empty view after submit")
	}
}

func TestIntegration_TokenCountsUpdate(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("response", "end_turn"),
	})

	th.Submit("count tokens")

	if th.Model.totalInputTokens == 0 {
		t.Error("expected non-zero input tokens after turn")
	}
	if th.Model.totalOutputTokens == 0 {
		t.Error("expected non-zero output tokens after turn")
	}
	if th.Model.turn != 1 {
		t.Errorf("expected turn 1, got %d", th.Model.turn)
	}
}

func TestIntegration_WindowResize(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("ok", "end_turn"),
	})

	// Resize.
	th.SendMsg(tea.WindowSizeMsg{Width: 200, Height: 50})

	if th.Model.width != 200 {
		t.Errorf("expected width 200, got %d", th.Model.width)
	}
	if th.Model.height != 50 {
		t.Errorf("expected height 50, got %d", th.Model.height)
	}

	// View should still render fine.
	_ = th.View()
}

// mockTool is a minimal tool implementation for integration tests.
type mockTool struct {
	name   string
	result tools.Result
}

func (m *mockTool) Name() string        { return m.name }
func (m *mockTool) Description() string { return "mock " + m.name }
func (m *mockTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (m *mockTool) Execute(_ context.Context, _ json.RawMessage) (tools.Result, error) {
	return m.result, nil
}

func TestIntegration_YoloModeToggle(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("ok", "end_turn"),
	})

	// Need a session for yolo toggle to work.
	th.Submit("init session")

	// Toggle yolo off via shift+tab.
	th.SendKey("shift+tab")
	if !th.OutputContains("YOLO mode disabled") {
		t.Error("expected yolo disabled message")
	}

	// Toggle back on.
	th.SendKey("shift+tab")
	if !th.OutputContains("YOLO mode enabled") {
		t.Error("expected yolo enabled message")
	}
}

func TestIntegration_EmptySubmitIgnored(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("should not see this", "end_turn"),
	})

	blocksBefore := th.BlockCount()
	th.update(SubmitMsg{Text: ""})

	// Should not add any blocks or change state.
	if th.BlockCount() != blocksBefore {
		t.Error("expected no new blocks from empty submit")
	}
	if th.State() != StateInput {
		t.Errorf("expected StateInput, got %d", th.State())
	}
}

func TestIntegration_MultipleResponsesSequence(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("alpha", "end_turn"),
		buildSSEResponse("beta", "end_turn"),
		buildSSEResponse("gamma", "end_turn"),
	})

	th.Submit("one")
	if !th.OutputContains("alpha") {
		t.Error("expected alpha")
	}

	th.Submit("two")
	if !th.OutputContains("beta") {
		t.Error("expected beta")
	}

	th.Submit("three")
	if !th.OutputContains("gamma") {
		t.Error("expected gamma")
	}

	if th.Model.turn != 3 {
		t.Errorf("expected 3 turns, got %d", th.Model.turn)
	}
}

func TestIntegration_LongResponseWithNewlines(t *testing.T) {
	var longText strings.Builder
	for i := 0; i < 50; i++ {
		longText.WriteString(fmt.Sprintf("Line %d of output\\n", i))
	}

	th := NewTestHarness(t, []string{
		buildSSEResponse(longText.String(), "end_turn"),
	})

	th.Submit("generate long output")

	if th.State() != StateInput {
		t.Errorf("expected StateInput, got %d", th.State())
	}

	// Should have text content.
	if th.BlocksOfKind(blockText) == 0 {
		t.Error("expected text blocks in output")
	}
}

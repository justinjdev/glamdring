package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

// --- Copy/Paste tests ---

// mockClipboard replaces the clipboard functions for testing and restores
// them when the test finishes.
type mockClipboard struct {
	text      string
	hasText   bool
	imageData []byte
	hasImage  bool
	written   string
}

func (mc *mockClipboard) install(t *testing.T) {
	t.Helper()
	origReadImage := ReadClipboardImage
	origReadText := ReadClipboardText
	origWrite := WriteClipboardText
	t.Cleanup(func() {
		ReadClipboardImage = origReadImage
		ReadClipboardText = origReadText
		WriteClipboardText = origWrite
	})

	ReadClipboardImage = func() ([]byte, bool) {
		return mc.imageData, mc.hasImage
	}
	ReadClipboardText = func() (string, bool) {
		return mc.text, mc.hasText
	}
	WriteClipboardText = func(text string) {
		mc.written = text
	}
}

// minimalPNG returns a minimal valid 1x1 PNG file (67 bytes).
func minimalPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
		0x00, 0x00, 0x00, 0x0d, // IHDR length
		0x49, 0x48, 0x44, 0x52, // "IHDR"
		0x00, 0x00, 0x00, 0x01, // width = 1
		0x00, 0x00, 0x00, 0x01, // height = 1
		0x08, 0x02, // bit depth=8, color type=2 (RGB)
		0x00, 0x00, 0x00,       // compression, filter, interlace
		0x90, 0x77, 0x53, 0xde, // IHDR CRC
		0x00, 0x00, 0x00, 0x0c, // IDAT length
		0x49, 0x44, 0x41, 0x54, // "IDAT"
		0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00, 0x00, // compressed data
		0x00, 0x02, 0x00, 0x01, // adler32
		0xe2, 0x21, 0xbc, 0x33, // IDAT CRC
		0x00, 0x00, 0x00, 0x00, // IEND length
		0x49, 0x45, 0x4e, 0x44, // "IEND"
		0xae, 0x42, 0x60, 0x82, // IEND CRC
	}
}

func TestIntegration_PasteText(t *testing.T) {
	mc := &mockClipboard{text: "pasted content", hasText: true}
	mc.install(t)

	th := NewTestHarness(t, []string{
		buildSSEResponse("ok", "end_turn"),
	})

	// Ctrl+V should insert the clipboard text into the textarea.
	th.SendKey("ctrl+v")

	// The textarea should now contain the pasted text.
	if th.Model.input.textarea.Value() != "pasted content" {
		t.Errorf("expected textarea to contain 'pasted content', got %q", th.Model.input.textarea.Value())
	}
}

func TestIntegration_PasteTextAndSubmit(t *testing.T) {
	mc := &mockClipboard{text: "pasted prompt", hasText: true}
	mc.install(t)

	th := NewTestHarness(t, []string{
		buildSSEResponse("I received your paste!", "end_turn"),
	})

	// Paste then submit.
	th.SendKey("ctrl+v")
	th.pending = nil
	th.update(SubmitMsg{Text: th.Model.input.textarea.Value()})
	th.DrainAgent()

	if !th.OutputContains("I received your paste!") {
		t.Error("expected agent response after pasting and submitting")
	}
	th.RequireViewContains("pasted prompt")
}

func TestIntegration_PasteImage(t *testing.T) {
	png := minimalPNG()
	mc := &mockClipboard{imageData: png, hasImage: true}
	mc.install(t)

	th := NewTestHarness(t, []string{
		buildSSEResponse("I see the image!", "end_turn"),
	})

	// Ctrl+V should stage the image.
	th.SendKey("ctrl+v")

	if len(th.Model.input.pendingImages) != 1 {
		t.Fatalf("expected 1 pending image, got %d", len(th.Model.input.pendingImages))
	}
	img := th.Model.input.pendingImages[0]
	if img.Width != 1 || img.Height != 1 {
		t.Errorf("expected 1x1 image, got %dx%d", img.Width, img.Height)
	}
}

func TestIntegration_PasteMultipleImages(t *testing.T) {
	png := minimalPNG()
	mc := &mockClipboard{imageData: png, hasImage: true}
	mc.install(t)

	th := NewTestHarness(t, []string{
		buildSSEResponse("ok", "end_turn"),
	})

	// Paste twice.
	th.SendKey("ctrl+v")
	th.SendKey("ctrl+v")

	if len(th.Model.input.pendingImages) != 2 {
		t.Fatalf("expected 2 pending images, got %d", len(th.Model.input.pendingImages))
	}
}

func TestIntegration_PasteNoClipboardData(t *testing.T) {
	mc := &mockClipboard{hasText: false, hasImage: false}
	mc.install(t)

	th := NewTestHarness(t, []string{
		buildSSEResponse("ok", "end_turn"),
	})

	// Ctrl+V with empty clipboard should be a no-op.
	th.SendKey("ctrl+v")

	if th.Model.input.textarea.Value() != "" {
		t.Errorf("expected empty textarea, got %q", th.Model.input.textarea.Value())
	}
	if len(th.Model.input.pendingImages) != 0 {
		t.Errorf("expected no pending images, got %d", len(th.Model.input.pendingImages))
	}
}

func TestIntegration_PasteImageFallbackToText(t *testing.T) {
	// Invalid image data (not PNG), but text is available.
	mc := &mockClipboard{
		imageData: []byte{0x00, 0x01, 0x02}, // not a valid PNG
		hasImage:  true,
		text:      "fallback text",
		hasText:   true,
	}
	mc.install(t)

	th := NewTestHarness(t, []string{
		buildSSEResponse("ok", "end_turn"),
	})

	th.SendKey("ctrl+v")

	// Should fall through to text paste since image is invalid PNG.
	if th.Model.input.textarea.Value() != "fallback text" {
		t.Errorf("expected textarea to contain 'fallback text', got %q", th.Model.input.textarea.Value())
	}
	if len(th.Model.input.pendingImages) != 0 {
		t.Errorf("expected no pending images for invalid PNG, got %d", len(th.Model.input.pendingImages))
	}
}

func TestIntegration_CopyCommand(t *testing.T) {
	mc := &mockClipboard{}
	mc.install(t)

	th := NewTestHarness(t, []string{
		buildSSEResponse("Response to copy", "end_turn"),
	})

	// Get a response first.
	th.Submit("say something")

	// /copy should copy the response to clipboard.
	th.update(SubmitMsg{Text: "/copy"})

	if mc.written != "Response to copy" {
		t.Errorf("expected clipboard to contain 'Response to copy', got %q", mc.written)
	}
	if !th.OutputContains("Copied") {
		t.Error("expected 'Copied' confirmation in output")
	}
}

func TestIntegration_CopyCommandNoResponse(t *testing.T) {
	mc := &mockClipboard{}
	mc.install(t)

	th := NewTestHarness(t, []string{
		buildSSEResponse("unused", "end_turn"),
	})

	// /copy with no prior response.
	th.update(SubmitMsg{Text: "/copy"})

	if mc.written != "" {
		t.Errorf("expected clipboard to be empty, got %q", mc.written)
	}
	if !th.OutputContains("No response to copy") {
		t.Error("expected 'No response to copy' error")
	}
}

func TestIntegration_PasteImageThenSubmit(t *testing.T) {
	png := minimalPNG()
	mc := &mockClipboard{imageData: png, hasImage: true}
	mc.install(t)

	th := NewTestHarness(t, []string{
		buildSSEResponse("I see your image!", "end_turn"),
	})

	// Paste image, then submit with text.
	th.SendKey("ctrl+v")
	if len(th.Model.input.pendingImages) != 1 {
		t.Fatal("expected 1 pending image after paste")
	}

	// Submit with text + image.
	th.pending = nil
	th.update(SubmitMsg{
		Text:   "describe this",
		Images: th.Model.input.pendingImages,
	})
	th.DrainAgent()

	if !th.OutputContains("I see your image!") {
		t.Error("expected agent response for image submission")
	}
	// View should show the image label.
	th.RequireViewContains("[Image 1: 1x1]")
}

func TestIntegration_LongResponseWithNewlines(t *testing.T) {
	var longText strings.Builder
	for i := range 50 {
		fmt.Fprintf(&longText, "Line %d of output\\n", i)
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

// --- Checkpoint tests ---

func TestIntegration_CheckpointAccept(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("ok", "end_turn"),
	})

	// Simulate a checkpoint being found at startup.
	th.SendMsg(checkpointFoundMsg{content: "Previous session context here"})

	if th.State() != StateCheckpoint {
		t.Fatalf("expected StateCheckpoint, got %d", th.State())
	}

	// View should show the checkpoint prompt.
	th.RequireViewContains("Load checkpoint")

	// Press 'y' to accept.
	th.SendKey("y")

	if th.State() != StateInput {
		t.Errorf("expected StateInput after accepting, got %d", th.State())
	}

	// System prompt should be augmented with checkpoint content.
	if !strings.Contains(th.Model.agentCfg.SystemPrompt, "Previous session context here") {
		t.Error("expected checkpoint content in system prompt")
	}
}

func TestIntegration_CheckpointReject(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("ok", "end_turn"),
	})

	th.SendMsg(checkpointFoundMsg{content: "Old context"})
	if th.State() != StateCheckpoint {
		t.Fatalf("expected StateCheckpoint, got %d", th.State())
	}

	// Press 'n' to reject.
	th.SendKey("n")

	if th.State() != StateInput {
		t.Errorf("expected StateInput after rejecting, got %d", th.State())
	}

	// System prompt should NOT contain checkpoint content.
	if strings.Contains(th.Model.agentCfg.SystemPrompt, "Old context") {
		t.Error("expected checkpoint content to NOT be in system prompt")
	}
}

func TestIntegration_CheckpointEscapeDismisses(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("ok", "end_turn"),
	})

	th.SendMsg(checkpointFoundMsg{content: "context"})
	th.SendKey("esc")

	if th.State() != StateInput {
		t.Errorf("expected StateInput after esc, got %d", th.State())
	}
}

// --- History navigation tests ---

func TestIntegration_HistoryUpDown(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("first reply", "end_turn"),
		buildSSEResponse("second reply", "end_turn"),
		buildSSEResponse("third reply", "end_turn"),
	})

	// Submit two prompts to build history.
	th.Submit("alpha prompt")
	th.Submit("beta prompt")

	// Press up arrow to recall previous command.
	th.SendKey("up")
	if th.Model.input.textarea.Value() != "beta prompt" {
		t.Errorf("expected 'beta prompt' after up, got %q", th.Model.input.textarea.Value())
	}

	// Press up again for older entry.
	th.SendKey("up")
	if th.Model.input.textarea.Value() != "alpha prompt" {
		t.Errorf("expected 'alpha prompt' after second up, got %q", th.Model.input.textarea.Value())
	}

	// Press down to go back.
	th.SendKey("down")
	if th.Model.input.textarea.Value() != "beta prompt" {
		t.Errorf("expected 'beta prompt' after down, got %q", th.Model.input.textarea.Value())
	}
}

// --- Ctrl+R search tests ---

func TestIntegration_CtrlRSearch(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("r1", "end_turn"),
		buildSSEResponse("r2", "end_turn"),
		buildSSEResponse("r3", "end_turn"),
	})

	// Build history.
	th.Submit("deploy to production")
	th.Submit("check logs")

	// Enter Ctrl+R search mode.
	th.SendKey("ctrl+r")
	if !th.Model.input.searching {
		t.Fatal("expected search mode to be active")
	}

	// Type search query.
	th.SendKey("d")
	th.SendKey("e")
	th.SendKey("p")

	// Accept match with Enter.
	th.SendKey("enter")
	if th.Model.input.searching {
		t.Error("expected search mode to be deactivated after enter")
	}

	// The textarea should contain the matched history entry.
	if th.Model.input.textarea.Value() != "deploy to production" {
		t.Errorf("expected 'deploy to production', got %q", th.Model.input.textarea.Value())
	}
}

func TestIntegration_CtrlRSearchCancel(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("r1", "end_turn"),
	})

	th.Submit("some command")

	th.SendKey("ctrl+r")
	th.SendKey("s")
	th.SendKey("esc")

	if th.Model.input.searching {
		t.Error("expected search mode to be deactivated after escape")
	}
	// Textarea should be empty (search cancelled).
	if th.Model.input.textarea.Value() != "" {
		t.Errorf("expected empty textarea after search cancel, got %q", th.Model.input.textarea.Value())
	}
}

// --- Tab completion tests ---

func TestIntegration_TabCompletion(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("ok", "end_turn"),
	})

	// Register builtin commands so tab completion has something to match.
	th.Model.input.SetAvailableCommands(BuiltinNames())

	// Type a partial slash command.
	for _, ch := range "/hel" {
		th.SendKey(string(ch))
	}

	// Tab should complete to /help.
	th.SendKey("tab")

	val := th.Model.input.textarea.Value()
	// Tab completion appends a trailing space after the completed command.
	if val != "/help " {
		t.Errorf("expected '/help ' after tab completion, got %q", val)
	}
}

// --- /compact flow test ---

func TestIntegration_CompactFlow(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("initial response", "end_turn"),
		buildSSEResponse("## Compacted Context\\nSummary of work done.", "end_turn"),
	})

	// Set CWD for checkpoint writing.
	tmpDir := t.TempDir()
	th.Model.agentCfg.CWD = tmpDir

	// First, get a response so we have context.
	th.Submit("do some work")

	if !th.OutputContains("initial response") {
		t.Fatal("expected initial response")
	}

	// Now run /compact -- this submits the compact prompt to the agent.
	th.pending = nil
	th.update(SubmitMsg{Text: "/compact"})

	if !th.Model.compacting {
		t.Fatal("expected compacting flag to be set")
	}
	if th.State() != StateRunning {
		t.Fatalf("expected StateRunning during compact, got %d", th.State())
	}

	// Drain the compact turn.
	th.DrainAgent()

	if th.State() != StateInput {
		t.Errorf("expected StateInput after compact, got %d", th.State())
	}

	// Output should have been cleared and show compact confirmation.
	if !th.OutputContains("Context compacted") {
		t.Error("expected 'Context compacted' message")
	}

	// Compacting flag should be cleared.
	if th.Model.compacting {
		t.Error("expected compacting to be false after compact completes")
	}
}

// --- /export flow test ---

func TestIntegration_ExportFlow(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("Export this response", "end_turn"),
	})

	// Get a response to export.
	th.Submit("say something")

	// Export to a temp file.
	tmpDir := t.TempDir()
	exportPath := tmpDir + "/test-export.md"

	th.update(SubmitMsg{Text: "/export " + exportPath})

	if !th.OutputContains("exported to") {
		t.Error("expected export confirmation")
	}

	// File should exist with content.
	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("failed to read export file: %v", err)
	}
	if !strings.Contains(string(data), "Export this response") {
		t.Errorf("expected response in exported file, got: %s", string(data))
	}
}

func TestIntegration_ExportHTML(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("HTML export test", "end_turn"),
	})

	th.Submit("say something")

	tmpDir := t.TempDir()
	exportPath := tmpDir + "/test-export.html"

	th.update(SubmitMsg{Text: "/export --html " + exportPath})

	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("failed to read export file: %v", err)
	}
	if !strings.Contains(string(data), "<!DOCTYPE html>") {
		t.Error("expected HTML doctype in exported file")
	}
	if !strings.Contains(string(data), "HTML export test") {
		t.Error("expected response content in HTML export")
	}
}

// --- Context threshold tests ---

func buildSSEResponseWithTokens(text string, inputTokens int) string {
	var b strings.Builder
	b.WriteString("event: message_start\n")
	fmt.Fprintf(&b, `data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"usage":{"input_tokens":%d,"output_tokens":0}}}`, inputTokens)
	b.WriteString("\n\n")

	b.WriteString("event: content_block_start\n")
	b.WriteString(`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
	b.WriteString("\n\n")

	b.WriteString("event: content_block_delta\n")
	fmt.Fprintf(&b, `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":%q}}`, text)
	b.WriteString("\n\n")

	b.WriteString("event: content_block_stop\n")
	b.WriteString(`data: {"type":"content_block_stop","index":0}`)
	b.WriteString("\n\n")

	b.WriteString("event: message_delta\n")
	b.WriteString(`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":50}}`)
	b.WriteString("\n\n")

	b.WriteString("event: message_stop\n")
	b.WriteString(`data: {"type":"message_stop"}`)
	b.WriteString("\n\n")

	return b.String()
}

func TestIntegration_ContextThreshold60(t *testing.T) {
	// 60% of 200,000 = 120,000 tokens
	th := NewTestHarness(t, []string{
		buildSSEResponseWithTokens("response", 120_000),
	})

	th.Submit("fill context")

	if !th.OutputContains("/compact available") {
		t.Error("expected 60% context threshold warning")
	}
}

func TestIntegration_ContextThreshold80(t *testing.T) {
	// 80% of 200,000 = 160,000 tokens
	th := NewTestHarness(t, []string{
		buildSSEResponseWithTokens("response", 160_000),
	})

	th.Submit("fill context more")

	if !th.OutputContains("consider running /compact") {
		t.Error("expected 80% context threshold warning")
	}
}

// --- Expand/collapse tool results ---

func TestIntegration_ExpandCollapseToolResult(t *testing.T) {
	// Generate a response with a large tool result that auto-collapses.
	th := NewTestHarness(t, []string{
		buildToolUseResponse("Read", "tu_1", `{"file_path":"/tmp/big.txt"}`),
		buildSSEResponse("Done reading", "end_turn"),
	})

	// Create a mock tool that returns a large result.
	var largeOutput strings.Builder
	for i := range 30 {
		fmt.Fprintf(&largeOutput, "line %d of output\n", i)
	}
	th.Model.agentCfg.Tools = []tools.Tool{&mockTool{
		name:   "Read",
		result: tools.Result{Output: largeOutput.String()},
	}}

	th.Submit("read the file")

	// Find the tool result block.
	var toolResultIdx int
	found := false
	for i, b := range th.Model.output.blocks {
		if b.kind == blockToolResult {
			toolResultIdx = i
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected a tool result block")
	}

	// Should be auto-collapsed (30 lines > threshold of 20).
	if !th.Model.output.collapsed[toolResultIdx] {
		t.Error("expected tool result to be auto-collapsed")
	}

	// Directly toggle (simulating 'e' key while in running state -- since
	// the agent is done, we test the toggle method directly).
	th.Model.output.ToggleLastToolResult()

	if th.Model.output.collapsed[toolResultIdx] {
		t.Error("expected tool result to be expanded after toggle")
	}
}

// --- /model command ---

func TestIntegration_ModelCommand(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("ok", "end_turn"),
	})

	// /model with no args opens config modal.
	th.update(SubmitMsg{Text: "/model"})
	if th.Model.state != StateModal {
		t.Error("expected StateModal after /model with no args")
	}
	if th.Model.modal == nil {
		t.Error("expected modal to be set")
	}
}

// --- /thinking toggle ---

func TestIntegration_ThinkingToggle(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("ok", "end_turn"),
	})

	if th.Model.showThinking {
		t.Error("expected thinking to be off by default")
	}

	th.update(SubmitMsg{Text: "/thinking"})
	if !th.Model.showThinking {
		t.Error("expected thinking to be on after toggle")
	}
	if !th.OutputContains("enabled") {
		t.Error("expected 'enabled' message")
	}

	th.update(SubmitMsg{Text: "/thinking"})
	if th.Model.showThinking {
		t.Error("expected thinking to be off after second toggle")
	}
}

// --- MCPServerDied message ---

func TestIntegration_MCPServerDied(t *testing.T) {
	th := NewTestHarness(t, []string{
		buildSSEResponse("ok", "end_turn"),
	})

	th.SendMsg(MCPServerDiedMsg{Name: "test-server"})

	if !th.OutputContains("test-server") {
		t.Error("expected MCP server name in error output")
	}
	if !th.OutputContains("died unexpectedly") {
		t.Error("expected 'died unexpectedly' error message")
	}
}

package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justin/glamdring/pkg/agent"
	"github.com/justin/glamdring/pkg/commands"
	"github.com/justin/glamdring/pkg/mcp"
)

func intPtr(v int) *int { return &v }

// newTestModel creates a minimal Model for testing built-in commands.
func newTestModel() Model {
	m := New()
	m.agentCfg = agent.Config{
		Model:    "claude-opus-4-6",
		MaxTurns: intPtr(10),
		CWD:      "/tmp/test",
	}
	m.totalInputTokens = 5000
	m.totalOutputTokens = 1000
	m.turn = 3
	return m
}

func TestDispatchBuiltin_Precedence(t *testing.T) {
	// Built-in commands should be found by dispatch.
	for _, name := range []string{"help", "quit", "clear", "cost", "config", "model", "compact"} {
		handler, ok := DispatchBuiltin(name)
		if !ok {
			t.Errorf("expected built-in %q to be found", name)
		}
		if handler == nil {
			t.Errorf("expected non-nil handler for %q", name)
		}
	}

	// Non-built-in should not be found.
	_, ok := DispatchBuiltin("review")
	if ok {
		t.Error("expected 'review' to not be a built-in")
	}
}

func TestBuiltinPrecedenceOverUserDefined(t *testing.T) {
	// Simulate a user-defined "help" command — built-in should take precedence.
	m := newTestModel()

	// Create a registry with a user-defined "help" command.
	reg := commands.NewRegistry([]commands.Command{
		{Name: "help", Path: "/tmp/help.md", Source: "user"},
	})
	m.SetCommandRegistry(reg)

	// Dispatch should find the built-in, not the user-defined one.
	handler, ok := DispatchBuiltin("help")
	if !ok {
		t.Fatal("expected built-in help to be found")
	}

	// The handler should append a system block (not expand a file).
	cmd := handler(&m, "")
	if cmd != nil {
		t.Error("expected nil cmd from /help")
	}

	// Verify output contains built-in commands list.
	if len(m.output.blocks) == 0 {
		t.Fatal("expected output blocks after /help")
	}
	last := m.output.blocks[len(m.output.blocks)-1]
	if last.kind != blockSystem {
		t.Errorf("expected blockSystem, got %d", last.kind)
	}
	if !strings.Contains(last.content, "/help") {
		t.Error("expected help output to list /help command")
	}
}

func TestCmdModel_NoArg(t *testing.T) {
	m := newTestModel()
	cmd := cmdModel(&m, "")
	if cmd != nil {
		t.Error("expected nil cmd")
	}

	if m.state != StateModal {
		t.Errorf("expected StateModal, got %d", m.state)
	}
	if m.modal == nil {
		t.Fatal("expected modal to be set")
	}
}

func TestCmdModel_WithArg(t *testing.T) {
	m := newTestModel()
	cmd := cmdModel(&m, "claude-sonnet-4-6")
	if cmd != nil {
		t.Error("expected nil cmd")
	}

	// Model should be updated.
	if m.agentCfg.Model != "claude-sonnet-4-6" {
		t.Errorf("expected model to be changed, got %q", m.agentCfg.Model)
	}

	// Statusbar should reflect new model.
	if m.statusbar.model != "claude-sonnet-4-6" {
		t.Errorf("expected statusbar model to be updated, got %q", m.statusbar.model)
	}
}

func TestCmdClear(t *testing.T) {
	m := newTestModel()

	// Add some output first.
	m.output.AppendUserMessage("hello")
	m.output.AppendText("response")
	m.output.FlushAllPending()

	if len(m.output.blocks) == 0 {
		t.Fatal("expected non-empty output before clear")
	}

	cmd := cmdClear(&m, "")
	if cmd != nil {
		t.Error("expected nil cmd")
	}

	// Output should be cleared.
	if len(m.output.blocks) != 0 {
		t.Errorf("expected 0 blocks after clear, got %d", len(m.output.blocks))
	}

	// Counters should be zeroed.
	if m.totalInputTokens != 0 {
		t.Errorf("expected 0 input tokens, got %d", m.totalInputTokens)
	}
	if m.totalOutputTokens != 0 {
		t.Errorf("expected 0 output tokens, got %d", m.totalOutputTokens)
	}
	if m.turn != 0 {
		t.Errorf("expected 0 turns, got %d", m.turn)
	}
	if m.statusbar.inputTokens != 0 {
		t.Errorf("expected statusbar input tokens 0, got %d", m.statusbar.inputTokens)
	}
}

func TestCmdQuit(t *testing.T) {
	m := newTestModel()
	cmd := cmdQuit(&m, "")
	if cmd == nil {
		t.Fatal("expected non-nil cmd from /quit")
	}

	// The cmd should produce a tea.QuitMsg.
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestCmdCost(t *testing.T) {
	m := newTestModel()
	cmd := cmdCost(&m, "")
	if cmd != nil {
		t.Error("expected nil cmd")
	}

	if len(m.output.blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(m.output.blocks))
	}
	content := m.output.blocks[0].content
	if !strings.Contains(content, "5.0k") {
		t.Errorf("expected input tokens in output, got %q", content)
	}
	if !strings.Contains(content, "1.0k") {
		t.Errorf("expected output tokens in output, got %q", content)
	}
	if !strings.Contains(content, "Turns:  3") {
		t.Errorf("expected turn count in output, got %q", content)
	}
}

func TestCmdConfig(t *testing.T) {
	m := newTestModel()
	cmd := cmdConfig(&m, "")
	if cmd != nil {
		t.Error("expected nil cmd")
	}

	if m.state != StateModal {
		t.Errorf("expected StateModal, got %d", m.state)
	}
	if m.modal == nil {
		t.Fatal("expected modal to be set")
	}
}

func TestCmdHelp(t *testing.T) {
	m := newTestModel()
	cmd := cmdHelp(&m, "")
	if cmd != nil {
		t.Error("expected nil cmd")
	}

	if len(m.output.blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(m.output.blocks))
	}
	content := m.output.blocks[0].content

	// Should list all built-in commands.
	for name := range builtinDescriptions {
		if !strings.Contains(content, "/"+name) {
			t.Errorf("expected /%s in help output", name)
		}
	}
}

func TestBuiltinNames(t *testing.T) {
	names := BuiltinNames()
	if len(names) != len(builtinCommands) {
		t.Errorf("expected %d names, got %d", len(builtinCommands), len(names))
	}

	// Should be sorted.
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("names not sorted: %q comes after %q", names[i], names[i-1])
		}
	}
}

func TestCmdThinking_Toggle(t *testing.T) {
	m := newTestModel()
	if m.showThinking {
		t.Fatal("expected showThinking to be false by default")
	}

	// Enable thinking.
	cmd := cmdThinking(&m, "")
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if !m.showThinking {
		t.Error("expected showThinking to be true after toggle")
	}
	if len(m.output.blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(m.output.blocks))
	}
	if !strings.Contains(m.output.blocks[0].content, "enabled") {
		t.Errorf("expected 'enabled' in output, got %q", m.output.blocks[0].content)
	}

	// Disable thinking.
	cmd = cmdThinking(&m, "")
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if m.showThinking {
		t.Error("expected showThinking to be false after second toggle")
	}
	if !strings.Contains(m.output.blocks[1].content, "disabled") {
		t.Errorf("expected 'disabled' in output, got %q", m.output.blocks[1].content)
	}
}

func TestCmdYolo_Toggle(t *testing.T) {
	m := newTestModel()
	// Create session so yolo has something to work with.
	m.session = agent.NewSession(m.agentCfg)

	// Enable yolo.
	cmd := cmdYolo(&m, "")
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if !m.session.IsYolo() {
		t.Error("expected yolo to be true after /yolo")
	}
	if !m.statusbar.yolo {
		t.Error("expected statusbar yolo to be true")
	}
	if len(m.output.blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(m.output.blocks))
	}
	if !strings.Contains(m.output.blocks[0].content, "enabled") {
		t.Errorf("expected 'enabled' in output, got %q", m.output.blocks[0].content)
	}

	// Disable yolo.
	cmd = cmdYolo(&m, "")
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if m.session.IsYolo() {
		t.Error("expected yolo to be false after second /yolo")
	}
	if m.statusbar.yolo {
		t.Error("expected statusbar yolo to be false")
	}
	if !strings.Contains(m.output.blocks[1].content, "disabled") {
		t.Errorf("expected 'disabled' in output, got %q", m.output.blocks[1].content)
	}
}

func TestCmdYolo_Scoped(t *testing.T) {
	m := newTestModel()
	m.session = agent.NewSession(m.agentCfg)

	cmd := cmdYolo(&m, "Bash,Write")
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	// Scoped yolo should NOT set global yolo flag.
	if m.session.IsYolo() {
		t.Error("expected global yolo to remain false for scoped /yolo")
	}
	if len(m.output.blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(m.output.blocks))
	}
	if !strings.Contains(m.output.blocks[0].content, "Bash") {
		t.Errorf("expected 'Bash' in output, got %q", m.output.blocks[0].content)
	}
}

func TestCmdYolo_CreatesSessionIfNil(t *testing.T) {
	m := newTestModel()
	if m.session != nil {
		t.Fatal("expected session to be nil initially")
	}

	cmd := cmdYolo(&m, "")
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if m.session == nil {
		t.Fatal("expected session to be created by /yolo")
	}
	if !m.session.IsYolo() {
		t.Error("expected yolo to be true")
	}
}

func TestCmdCost_PerModelPricing(t *testing.T) {
	m := newTestModel()
	m.agentCfg.Model = "claude-sonnet-4-6"
	cmd := cmdCost(&m, "")
	if cmd != nil {
		t.Error("expected nil cmd")
	}

	content := m.output.blocks[0].content
	// With 5000 input and 1000 output at Sonnet pricing (3.0/15.0):
	// cost = 5000/1M * 3.0 + 1000/1M * 15.0 = 0.015 + 0.015 = 0.0300
	if !strings.Contains(content, "$0.0300") {
		t.Errorf("expected Sonnet pricing in cost output, got %q", content)
	}
}

// --- MCP command tests ---

// newTestMCPManager creates a Manager with a fake server for testing.
func newTestMCPManager() *mcp.Manager {
	mgr := mcp.NewManager()
	mgr.AddTestServer("test-server", []string{"tool_a", "tool_b"})
	return mgr
}

func TestCmdMCP_NilManager(t *testing.T) {
	m := newTestModel()
	m.mcpMgr = nil

	cmd := cmdMCP(&m, "")
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if len(m.output.blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(m.output.blocks))
	}
	if !strings.Contains(m.output.blocks[0].content, "No MCP servers configured") {
		t.Errorf("expected 'No MCP servers configured', got %q", m.output.blocks[0].content)
	}
}

func TestCmdMCP_UnknownSubcommand(t *testing.T) {
	m := newTestModel()
	m.mcpMgr = newTestMCPManager()

	cmd := cmdMCP(&m, "bogus")
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	found := false
	for _, b := range m.output.blocks {
		if b.kind == blockError && strings.Contains(b.content, "Unknown /mcp subcommand") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error about unknown subcommand")
	}
}

func TestCmdMCP_List(t *testing.T) {
	m := newTestModel()
	m.mcpMgr = newTestMCPManager()

	cmd := cmdMCP(&m, "")
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if len(m.output.blocks) == 0 {
		t.Fatal("expected output blocks")
	}
	content := m.output.blocks[0].content
	if !strings.Contains(content, "test-server") {
		t.Errorf("expected server name in output, got %q", content)
	}
	if !strings.Contains(content, "2 tools") {
		t.Errorf("expected tool count in output, got %q", content)
	}
}

func TestCmdMCP_Tools(t *testing.T) {
	m := newTestModel()
	m.mcpMgr = newTestMCPManager()

	cmd := cmdMCP(&m, "tools test-server")
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if len(m.output.blocks) == 0 {
		t.Fatal("expected output blocks")
	}
	content := m.output.blocks[0].content
	if !strings.Contains(content, "tool_a") || !strings.Contains(content, "tool_b") {
		t.Errorf("expected tool names in output, got %q", content)
	}
}

func TestCmdMCP_DisableTool(t *testing.T) {
	m := newTestModel()
	m.mcpMgr = newTestMCPManager()

	cmd := cmdMCP(&m, "disable test-server tool_a")
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	found := false
	for _, b := range m.output.blocks {
		if b.kind == blockSystem && strings.Contains(b.content, "Disabled") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected confirmation of disable")
	}
}

func TestCmdMCP_EnableTool(t *testing.T) {
	m := newTestModel()
	m.mcpMgr = newTestMCPManager()

	// Disable first, then enable.
	_ = m.mcpMgr.DisableTool("test-server", "tool_a")
	cmd := cmdMCP(&m, "enable test-server tool_a")
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	found := false
	for _, b := range m.output.blocks {
		if b.kind == blockSystem && strings.Contains(b.content, "Enabled") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected confirmation of enable")
	}
}

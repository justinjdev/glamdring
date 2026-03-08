package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justin/glamdring/pkg/agent"
)

func TestEscapePermissionResumesAgent(t *testing.T) {
	m := New()
	m.state = StatePermission
	permCh := make(chan agent.PermissionAnswer, 1)
	am := agent.Message{
		Type:               agent.MessagePermissionRequest,
		PermissionResponse: permCh,
	}
	m.permission = &am
	agentCh := make(chan agent.Message, 1)
	m.agentCh = agentCh

	// Press Escape to deny.
	result, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyEscape})
	model := result.(Model)

	// State should be Running (denyPermission sets it).
	if model.state != StateRunning {
		t.Errorf("expected StateRunning, got %d", model.state)
	}
	// Permission should be cleared.
	if model.permission != nil {
		t.Error("expected permission to be nil after escape")
	}
	// Cmd should be non-nil (waitForAgent resumes draining).
	if cmd == nil {
		t.Error("expected non-nil cmd to resume agent draining")
	}
	// Permission channel should have received deny.
	answer := <-permCh
	if answer != agent.PermissionDeny {
		t.Errorf("expected PermissionDeny, got %d", answer)
	}
}

func TestCtrlCInterruptsAgent(t *testing.T) {
	m := New()
	m.state = StateRunning
	cancelled := false
	m.cancelTurn = func() { cancelled = true }
	m.spinning = true
	agentCh := make(chan agent.Message, 1)
	m.agentCh = agentCh

	result, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyCtrlC})
	model := result.(Model)

	if model.state != StateInput {
		t.Errorf("expected StateInput after interrupt, got %d", model.state)
	}
	if !cancelled {
		t.Error("expected cancelTurn to be called")
	}
	if model.spinning {
		t.Error("expected spinning to be false after interrupt")
	}
	if model.agentCh != nil {
		t.Error("expected agentCh to be nil after interrupt")
	}
	// Should have appended "(interrupted)" system message.
	found := false
	for _, b := range model.output.blocks {
		if b.kind == blockSystem && b.content == "(interrupted)" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected '(interrupted)' system message in output")
	}
	// cmd should be non-nil (focus input).
	if cmd == nil {
		t.Error("expected non-nil cmd for focus")
	}
}

func TestDoubleCtrlCQuits(t *testing.T) {
	m := New()
	m.state = StateInput
	m.lastCtrlC = time.Now() // simulate first press just happened

	result, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyCtrlC})
	_ = result

	// cmd should produce a tea.QuitMsg.
	if cmd == nil {
		t.Fatal("expected non-nil cmd for quit")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestSingleCtrlCShowsHint(t *testing.T) {
	m := New()
	m.state = StateInput

	result, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyCtrlC})
	model := result.(Model)

	if model.lastCtrlC.IsZero() {
		t.Error("expected lastCtrlC to be set")
	}
	found := false
	for _, b := range model.output.blocks {
		if b.kind == blockSystem && b.content == "(press Ctrl+C again to quit)" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected quit hint in output")
	}
}

func TestSpinnerStopsOnTextDelta(t *testing.T) {
	m := New()
	m.spinning = true

	agentMsg := AgentMsg(agent.Message{
		Type: agent.MessageTextDelta,
		Text: "hello",
	})
	result, _ := m.handleAgentMsg(agentMsg)
	if result.spinning {
		t.Error("expected spinning to stop on text delta")
	}
}

func TestToolCallShowsInlineSpinner(t *testing.T) {
	m := New()
	m.spinning = true
	m.spinnerLabel = "Thinking..."

	agentMsg := AgentMsg(agent.Message{
		Type:     agent.MessageToolCall,
		ToolName: "Read",
		ToolInput: map[string]any{"file_path": "/tmp/test"},
	})
	result, _ := m.handleAgentMsg(agentMsg)
	if !result.spinning {
		t.Error("expected spinning to continue during tool execution")
	}
	// Spinner label is empty because the spinner is rendered inline
	// on the tool call block, not as a separate line.
	if result.spinnerLabel != "" {
		t.Errorf("expected empty spinner label for inline tool spinner, got %q", result.spinnerLabel)
	}
	if result.output.toolSpinner == "" {
		t.Error("expected inline tool spinner to be set")
	}
}

func TestSpinnerStopsOnError(t *testing.T) {
	m := New()
	m.spinning = true

	agentMsg := AgentMsg(agent.Message{
		Type: agent.MessageError,
		Err:  nil,
	})
	result, _ := m.handleAgentMsg(agentMsg)
	if result.spinning {
		t.Error("expected spinning to stop on error")
	}
}

func TestSummarizeToolInput(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    map[string]any
		want     string
	}{
		{
			name:     "Bash command",
			toolName: "Bash",
			input:    map[string]any{"command": "ls -la"},
			want:     "ls -la",
		},
		{
			name:     "Read file path",
			toolName: "Read",
			input:    map[string]any{"file_path": "/tmp/test.go"},
			want:     "/tmp/test.go",
		},
		{
			name:     "Write file path",
			toolName: "Write",
			input:    map[string]any{"file_path": "/tmp/out.go"},
			want:     "/tmp/out.go",
		},
		{
			name:     "Edit file path",
			toolName: "Edit",
			input:    map[string]any{"file_path": "/tmp/edit.go"},
			want:     "/tmp/edit.go",
		},
		{
			name:     "Glob pattern",
			toolName: "Glob",
			input:    map[string]any{"pattern": "**/*.go"},
			want:     "**/*.go",
		},
		{
			name:     "Grep pattern",
			toolName: "Grep",
			input:    map[string]any{"pattern": "func main"},
			want:     "func main",
		},
		{
			name:     "unknown tool with input",
			toolName: "Custom",
			input:    map[string]any{"key": "value"},
			want:     "key=value",
		},
		{
			name:     "no input",
			toolName: "Custom",
			input:    map[string]any{},
			want:     "(no input)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeToolInput(tt.toolName, tt.input)
			if got != tt.want {
				t.Errorf("summarizeToolInput(%q, ...) = %q, want %q", tt.toolName, got, tt.want)
			}
		})
	}
}

func TestSummarizeToolInput_LongBashCommand(t *testing.T) {
	longCmd := strings.Repeat("x", 100)
	got := summarizeToolInput("Bash", map[string]any{"command": longCmd})
	if len(got) > 80 {
		t.Errorf("expected truncated output (<=80 chars), got length %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("expected '...' suffix for truncated command")
	}
}

func TestExtractLastText(t *testing.T) {
	m := New()
	// No blocks -- should return empty.
	if text := m.extractLastText(); text != "" {
		t.Errorf("expected empty string, got %q", text)
	}

	// Add some blocks.
	m.output.AppendText("first text")
	m.output.FlushAllPending()
	m.output.AppendToolCall("Read", "file.go")
	m.output.AppendText("second text")
	m.output.FlushAllPending()

	text := m.extractLastText()
	if text != "second text" {
		t.Errorf("expected 'second text', got %q", text)
	}
}

func TestLayoutComponents(t *testing.T) {
	m := New()
	m.width = 120
	m.height = 40
	m.layoutComponents()

	// Verify components were sized.
	if m.output.viewport.Width != 120 {
		t.Errorf("expected output width 120, got %d", m.output.viewport.Width)
	}
	if m.output.viewport.Height < 1 {
		t.Error("expected positive output height")
	}
}

func TestBuildContentBlocks_ImagesAndText(t *testing.T) {
	images := []PendingImage{
		{Data: []byte{0x89, 0x50, 0x4E, 0x47}, Width: 100, Height: 200},
	}
	blocks := buildContentBlocks("describe this", images)

	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Type != "image" {
		t.Errorf("first block type = %q, want 'image'", blocks[0].Type)
	}
	if blocks[0].Source == nil {
		t.Fatal("expected non-nil source on image block")
	}
	if blocks[0].Source.MediaType != "image/png" {
		t.Errorf("media_type = %q, want 'image/png'", blocks[0].Source.MediaType)
	}
	if blocks[1].Type != "text" {
		t.Errorf("second block type = %q, want 'text'", blocks[1].Type)
	}
	if blocks[1].Text != "describe this" {
		t.Errorf("text = %q, want 'describe this'", blocks[1].Text)
	}
}

func TestBuildContentBlocks_ImagesOnly(t *testing.T) {
	images := []PendingImage{
		{Data: []byte{1}},
		{Data: []byte{2}},
	}
	blocks := buildContentBlocks("", images)

	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks (no text), got %d", len(blocks))
	}
	for _, b := range blocks {
		if b.Type != "image" {
			t.Errorf("expected all image blocks, got %q", b.Type)
		}
	}
}

func TestBuildContentBlocks_TextOnly(t *testing.T) {
	blocks := buildContentBlocks("hello", nil)

	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Type != "text" || blocks[0].Text != "hello" {
		t.Errorf("expected text block with 'hello'")
	}
}

func TestExpandCollapseKeyToggle(t *testing.T) {
	m := New()
	m.state = StateRunning

	// Add a large tool result that gets auto-collapsed.
	longOutput := ""
	for i := 0; i < 30; i++ {
		longOutput += "line\n"
	}
	m.output.AppendToolResult(longOutput, false)

	// The block should be auto-collapsed (30 > collapseThreshold).
	blockIdx := len(m.output.blocks) - 1
	if !m.output.collapsed[blockIdx] {
		t.Fatal("expected tool result to be auto-collapsed")
	}

	// Press 'e' to toggle.
	result, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	model := result.(Model)

	if model.output.collapsed[blockIdx] {
		t.Error("expected tool result to be expanded after 'e' key")
	}
}

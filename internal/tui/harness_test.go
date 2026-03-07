package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justin/glamdring/pkg/agent"
	"github.com/justin/glamdring/pkg/auth"
)

// TestHarness drives the TUI Model through its full Init -> Update -> View
// cycle backed by a mock API server. It simulates what the Bubble Tea runtime
// does: executing returned commands, feeding their results back through Update.
type TestHarness struct {
	t       *testing.T
	Model   Model
	server  *httptest.Server
	pending []tea.Cmd
}

// mockCreds implements auth.Credentials for testing.
type mockCreds struct{}

func (mockCreds) SetAuthHeaders(r *http.Request) error {
	r.Header.Set("x-api-key", "test-key")
	return nil
}
func (mockCreds) IsOAuth() bool { return false }

var _ auth.Credentials = mockCreds{}

// HarnessOpt configures a TestHarness.
type HarnessOpt func(*TestHarness)

// WithSize sets the terminal dimensions.
func WithSize(w, h int) HarnessOpt {
	return func(th *TestHarness) {
		th.Model.width = w
		th.Model.height = h
		th.Model.layoutComponents()
	}
}

// buildSSEResponse constructs a complete SSE stream for a simple text response.
func buildSSEResponse(text, stopReason string) string {
	var b strings.Builder

	b.WriteString("event: message_start\n")
	b.WriteString(`data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":0}}}`)
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
	fmt.Fprintf(&b, `data: {"type":"message_delta","delta":{"stop_reason":"%s"},"usage":{"output_tokens":50}}`, stopReason)
	b.WriteString("\n\n")

	b.WriteString("event: message_stop\n")
	b.WriteString(`data: {"type":"message_stop"}`)
	b.WriteString("\n\n")

	return b.String()
}

// buildToolUseResponse constructs an SSE stream where the assistant calls a tool.
func buildToolUseResponse(toolName, toolID, inputJSON string) string {
	var b strings.Builder

	b.WriteString("event: message_start\n")
	b.WriteString(`data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"usage":{"input_tokens":100,"output_tokens":0}}}`)
	b.WriteString("\n\n")

	b.WriteString("event: content_block_start\n")
	fmt.Fprintf(&b, `data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"%s","name":"%s"}}`, toolID, toolName)
	b.WriteString("\n\n")

	b.WriteString("event: content_block_delta\n")
	fmt.Fprintf(&b, `data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"%s"}}`, strings.ReplaceAll(inputJSON, `"`, `\"`))
	b.WriteString("\n\n")

	b.WriteString("event: content_block_stop\n")
	b.WriteString(`data: {"type":"content_block_stop","index":0}`)
	b.WriteString("\n\n")

	b.WriteString("event: message_delta\n")
	b.WriteString(`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":20}}`)
	b.WriteString("\n\n")

	b.WriteString("event: message_stop\n")
	b.WriteString(`data: {"type":"message_stop"}`)
	b.WriteString("\n\n")

	return b.String()
}

// NewTestHarness creates a TestHarness backed by a mock API server that serves
// the given SSE responses in order. The harness is initialized with an 80x24
// terminal and runs Init.
func NewTestHarness(t *testing.T, responses []string, opts ...HarnessOpt) *TestHarness {
	t.Helper()

	var call atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := int(call.Load())
		if idx >= len(responses) {
			idx = len(responses) - 1
		}
		call.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, responses[idx])
	}))
	t.Cleanup(srv.Close)

	ctx := context.Background()
	cfg := agent.Config{
		Model:    "test-model",
		Creds:    mockCreds{},
		Endpoint: srv.URL,
		Yolo:     true, // auto-approve tools for integration tests
	}

	m := NewWithAgent(ctx, cfg)

	th := &TestHarness{
		t:      t,
		Model:  m,
		server: srv,
	}

	// Default size.
	th.Model.width = 80
	th.Model.height = 24
	th.Model.layoutComponents()

	for _, opt := range opts {
		opt(th)
	}

	// Run Init but discard the resulting commands. Init produces timer-based
	// commands (spinner tick, cursor blink) that would block or loop forever
	// in a synchronous test harness. The model is already usable without them.
	_ = th.Model.Init()

	return th
}

// SendKey sends a key press through Update.
func (th *TestHarness) SendKey(key string) {
	th.t.Helper()
	msg := parseKeyMsg(key)
	th.update(msg)
}

// SendMsg sends an arbitrary tea.Msg through Update.
func (th *TestHarness) SendMsg(msg tea.Msg) {
	th.t.Helper()
	th.update(msg)
}

// Submit simulates the user typing text and pressing enter. It sends a
// SubmitMsg through Update, then drains the agent until the turn completes.
func (th *TestHarness) Submit(text string) {
	th.t.Helper()
	// Clear stale pending commands from previous operations (e.g., agentDoneMsg
	// from a prior turn that was queued after DrainAgent returned on StateInput).
	th.pending = nil
	th.update(SubmitMsg{Text: text})
	th.DrainAgent()
}

// DrainAgent executes pending commands and feeds results back through Update
// until the model returns to StateInput (agent turn complete). This handles
// the full command chain: agentStartedMsg -> waitForAgent -> AgentMsg -> ... -> agentDoneMsg.
//
// Commands are executed concurrently with a timeout to skip timer-based
// commands (spinner ticks, cursor blinks) that would block indefinitely.
func (th *TestHarness) DrainAgent() {
	th.t.Helper()
	const maxIterations = 500
	const cmdTimeout = 5 * time.Second
	for i := range maxIterations {
		if len(th.pending) == 0 {
			return
		}

		// Execute all pending commands concurrently with a timeout.
		// Timer-based commands (spinner, blink) block for their interval
		// and would make tests unbearably slow if executed sequentially.
		type cmdResult struct {
			msg tea.Msg
		}
		results := make(chan cmdResult, len(th.pending))
		cmdCount := 0
		for _, cmd := range th.pending {
			if cmd == nil {
				continue
			}
			cmdCount++
			go func(c tea.Cmd) {
				results <- cmdResult{msg: c()}
			}(cmd)
		}
		th.pending = nil

		// Collect results with timeout.
		deadline := time.After(cmdTimeout)
		var msgs []tea.Msg
		collected := 0
		for collected < cmdCount {
			select {
			case r := <-results:
				collected++
				if r.msg == nil {
					continue
				}
				// Unwrap BatchMsg into more commands.
				if batch, ok := r.msg.(tea.BatchMsg); ok {
					for _, c := range batch {
						if c != nil {
							th.pending = append(th.pending, c)
						}
					}
				} else {
					msgs = append(msgs, r.msg)
				}
			case <-deadline:
				// Some commands timed out (likely timers). Skip them.
				goto done
			}
		}
	done:

		if len(msgs) == 0 && len(th.pending) == 0 {
			return
		}

		// Feed results through Update. Discard any timer-produced commands
		// (spinner tick -> another tick) to prevent infinite loops.
		for _, msg := range msgs {
			result, cmd := th.Model.Update(msg)
			th.Model = result.(Model)
			if cmd != nil {
				th.collectNonTimerCmds(cmd)
			}
		}

		// If we're back in input state, we're done.
		if th.Model.state == StateInput {
			return
		}

		if i == maxIterations-1 {
			th.t.Fatal("DrainAgent: exceeded max iterations")
		}
	}
}

// View returns the current rendered view.
func (th *TestHarness) View() string {
	return th.Model.View()
}

// ViewContains checks if the rendered view contains the given substring.
func (th *TestHarness) ViewContains(s string) bool {
	return strings.Contains(th.View(), s)
}

// RequireViewContains fails the test if the view does not contain the substring.
func (th *TestHarness) RequireViewContains(s string) {
	th.t.Helper()
	if !th.ViewContains(s) {
		th.t.Errorf("expected view to contain %q, got:\n%s", s, th.View())
	}
}

// RequireViewNotContains fails the test if the view contains the substring.
func (th *TestHarness) RequireViewNotContains(s string) {
	th.t.Helper()
	if th.ViewContains(s) {
		th.t.Errorf("expected view to NOT contain %q", s)
	}
}

// State returns the current TUI state.
func (th *TestHarness) State() State {
	return th.Model.state
}

// OutputText returns concatenated text from all text output blocks.
func (th *TestHarness) OutputText() string {
	var parts []string
	for _, b := range th.Model.output.blocks {
		if b.kind == blockText {
			parts = append(parts, b.content)
		}
	}
	return strings.Join(parts, "")
}

// OutputContains checks if any output block contains the given substring.
func (th *TestHarness) OutputContains(s string) bool {
	for _, b := range th.Model.output.blocks {
		if strings.Contains(b.content, s) {
			return true
		}
	}
	return false
}

// BlockCount returns the number of output blocks.
func (th *TestHarness) BlockCount() int {
	return len(th.Model.output.blocks)
}

// BlocksOfKind returns the count of blocks matching the given kind.
func (th *TestHarness) BlocksOfKind(kind blockKind) int {
	count := 0
	for _, b := range th.Model.output.blocks {
		if b.kind == kind {
			count++
		}
	}
	return count
}

// update feeds a message through the Model's Update and collects returned commands.
func (th *TestHarness) update(msg tea.Msg) {
	result, cmd := th.Model.Update(msg)
	th.Model = result.(Model)
	if cmd != nil {
		th.collectNonTimerCmds(cmd)
	}
}

// collectNonTimerCmds extracts individual commands from a potentially batched
// command, filtering out timer-based commands (spinner ticks, cursor blinks)
// that would cause infinite loops in the synchronous test harness.
func (th *TestHarness) collectNonTimerCmds(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	// Execute to see if it produces a BatchMsg.
	msg := cmd()
	switch m := msg.(type) {
	case tea.BatchMsg:
		for _, c := range m {
			if c != nil {
				// Recursively unwrap nested batches.
				th.collectNonTimerCmds(c)
			}
		}
	default:
		if msg == nil {
			return
		}
		// Skip spinner tick messages -- they produce an infinite chain
		// of tick -> update -> tick commands.
		typeName := fmt.Sprintf("%T", msg)
		if strings.Contains(typeName, "TickMsg") || strings.Contains(typeName, "tickMsg") ||
			strings.Contains(typeName, "blinkMsg") || strings.Contains(typeName, "BlinkMsg") {
			return
		}
		th.pending = append(th.pending, func() tea.Msg { return msg })
	}
}

// parseKeyMsg converts a key string to a tea.KeyMsg.
func parseKeyMsg(key string) tea.KeyMsg {
	switch key {
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "ctrl+v":
		return tea.KeyMsg{Type: tea.KeyCtrlV}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/justin/glamdring/pkg/api"
	"github.com/justin/glamdring/pkg/config"
	"github.com/justin/glamdring/pkg/hooks"
	"github.com/justin/glamdring/pkg/tools"
)

// --- mock tool that can be configured to return errors ---

type configurableMockTool struct {
	name      string
	result    tools.Result
	execErr   error
	execCount int
}

func (t *configurableMockTool) Name() string        { return t.name }
func (t *configurableMockTool) Description() string  { return "configurable mock" }
func (t *configurableMockTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (t *configurableMockTool) Execute(_ context.Context, _ json.RawMessage) (tools.Result, error) {
	t.execCount++
	return t.result, t.execErr
}

// --- Tests for isAllowed ---

func TestIsAllowed(t *testing.T) {
	tests := []struct {
		name         string
		toolName     string
		sessionAllow map[string]bool
		want         bool
	}{
		{
			name:         "always-allow tool Read",
			toolName:     "Read",
			sessionAllow: nil,
			want:         true,
		},
		{
			name:         "always-allow tool Glob",
			toolName:     "Glob",
			sessionAllow: nil,
			want:         true,
		},
		{
			name:         "always-allow tool Grep",
			toolName:     "Grep",
			sessionAllow: nil,
			want:         true,
		},
		{
			name:         "session-approved tool",
			toolName:     "Bash",
			sessionAllow: map[string]bool{"Bash": true},
			want:         true,
		},
		{
			name:         "denied tool not in session or always-allow",
			toolName:     "Bash",
			sessionAllow: map[string]bool{},
			want:         false,
		},
		{
			name:         "denied tool with nil sessionAllow",
			toolName:     "Write",
			sessionAllow: nil,
			want:         false,
		},
		{
			name:         "unknown tool denied",
			toolName:     "Unknown",
			sessionAllow: map[string]bool{"Bash": true},
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAllowed(tt.toolName, tt.sessionAllow)
			if got != tt.want {
				t.Errorf("isAllowed(%q) = %v, want %v", tt.toolName, got, tt.want)
			}
		})
	}
}

// --- Tests for permissionSummary ---

func TestPermissionSummary(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    map[string]any
		want     string
	}{
		{
			name:     "Bash with short command",
			toolName: "Bash",
			input:    map[string]any{"command": "ls -la"},
			want:     "Run: ls -la",
		},
		{
			name:     "Bash with long command truncated",
			toolName: "Bash",
			input:    map[string]any{"command": strings.Repeat("x", 100)},
			want:     "Run: " + strings.Repeat("x", 77) + "...",
		},
		{
			name:     "Bash with exactly 80 chars",
			toolName: "Bash",
			input:    map[string]any{"command": strings.Repeat("y", 80)},
			want:     "Run: " + strings.Repeat("y", 80),
		},
		{
			name:     "Bash with exactly 81 chars triggers truncation",
			toolName: "Bash",
			input:    map[string]any{"command": strings.Repeat("z", 81)},
			want:     "Run: " + strings.Repeat("z", 77) + "...",
		},
		{
			name:     "Bash with no command field",
			toolName: "Bash",
			input:    map[string]any{},
			want:     "Run: Bash",
		},
		{
			name:     "Write with file_path",
			toolName: "Write",
			input:    map[string]any{"file_path": "/tmp/test.txt"},
			want:     "Write to /tmp/test.txt",
		},
		{
			name:     "Write without file_path",
			toolName: "Write",
			input:    map[string]any{},
			want:     "Write to file",
		},
		{
			name:     "Edit with file_path",
			toolName: "Edit",
			input:    map[string]any{"file_path": "/src/main.go"},
			want:     "Edit /src/main.go",
		},
		{
			name:     "Edit without file_path",
			toolName: "Edit",
			input:    map[string]any{},
			want:     "Edit file",
		},
		{
			name:     "unknown tool",
			toolName: "CustomTool",
			input:    map[string]any{},
			want:     "Execute tool: CustomTool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := permissionSummary(tt.toolName, tt.input)
			if got != tt.want {
				t.Errorf("permissionSummary(%q) = %q, want %q", tt.toolName, got, tt.want)
			}
		})
	}
}

// --- Tests for truncateToolResult (additional edge cases) ---

func TestTruncateToolResult_UTF8Boundary(t *testing.T) {
	// Build a string where a multi-byte rune straddles the maxToolResultSize boundary.
	// The emoji is 4 bytes. Place it so bytes [maxToolResultSize-2..maxToolResultSize+1]
	// are the emoji, causing truncation to split it.
	prefix := strings.Repeat("a", maxToolResultSize-2)
	input := prefix + "\xf0\x9f\x98\x80" // 4-byte emoji
	result := truncateToolResult(input)

	if !strings.Contains(result, "truncated") {
		t.Error("expected truncation notice")
	}
	// The result before the notice should be valid UTF-8.
	beforeNotice := strings.SplitN(result, "\n... (truncated", 2)[0]
	for i := 0; i < len(beforeNotice); {
		r, size := rune(beforeNotice[i]), 0
		for _, b := range []byte(beforeNotice[i:]) {
			_ = b
			size++
			if size >= 4 {
				break
			}
		}
		if r == '\uFFFD' {
			t.Error("found replacement character in truncated result, expected clean UTF-8")
			break
		}
		i += size
	}
}

// --- Tests for executeTools ---

func makeToolCall(id, name string, input map[string]any) toolCall {
	raw, _ := json.Marshal(input)
	return toolCall{id: id, name: name, input: raw}
}

func TestExecuteTools_PermissionDeny(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&configurableMockTool{
		name:   "Bash",
		result: tools.Result{Output: "should not run"},
	})

	perms := &config.PermissionConfig{
		Deny: []config.PermissionRule{{Tool: "Bash"}},
	}

	ctx := context.Background()
	out := make(chan Message, 64)
	calls := []toolCall{makeToolCall("tc1", "Bash", map[string]any{"command": "rm -rf /"})}

	results, err := executeTools(ctx, out, registry, calls, nil, nil, perms)
	close(out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].IsError {
		t.Error("expected error result for denied tool")
	}
	if results[0].Content != "blocked by permission rule" {
		t.Errorf("unexpected content: %q", results[0].Content)
	}

	// Verify messages were emitted.
	var msgs []Message
	for m := range out {
		msgs = append(msgs, m)
	}
	// Should have tool call + tool result.
	var gotToolCall, gotToolResult bool
	for _, m := range msgs {
		if m.Type == MessageToolCall {
			gotToolCall = true
		}
		if m.Type == MessageToolResult && m.ToolIsError {
			gotToolResult = true
		}
	}
	if !gotToolCall {
		t.Error("expected MessageToolCall emission")
	}
	if !gotToolResult {
		t.Error("expected MessageToolResult with error")
	}
}

func TestExecuteTools_PermissionAllow(t *testing.T) {
	mockT := &configurableMockTool{
		name:   "Write",
		result: tools.Result{Output: "file written"},
	}
	registry := tools.NewRegistry()
	registry.Register(mockT)

	perms := &config.PermissionConfig{
		Allow: []config.PermissionRule{{Tool: "Write"}},
	}

	ctx := context.Background()
	out := make(chan Message, 64)
	calls := []toolCall{makeToolCall("tc1", "Write", map[string]any{"file_path": "/tmp/x"})}

	results, err := executeTools(ctx, out, registry, calls, map[string]bool{}, nil, perms)
	close(out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].IsError {
		t.Error("expected successful result")
	}
	if results[0].Content != "file written" {
		t.Errorf("unexpected content: %q", results[0].Content)
	}
	if mockT.execCount != 1 {
		t.Errorf("expected tool to be executed once, got %d", mockT.execCount)
	}
}

func TestExecuteTools_HookBlock(t *testing.T) {
	mockT := &configurableMockTool{
		name:   "Bash",
		result: tools.Result{Output: "should not run"},
	}
	registry := tools.NewRegistry()
	registry.Register(mockT)

	// Create a hook runner with a PreToolUse hook that always fails.
	hookRunner := hooks.NewHookRunner([]hooks.Hook{
		{Event: hooks.PreToolUse, Command: "exit 1"},
	})

	ctx := context.Background()
	out := make(chan Message, 64)
	calls := []toolCall{makeToolCall("tc1", "Bash", map[string]any{"command": "echo hi"})}

	results, err := executeTools(ctx, out, registry, calls, map[string]bool{"Bash": true}, hookRunner, nil)
	close(out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].IsError {
		t.Error("expected error result when hook blocks")
	}
	if !strings.Contains(results[0].Content, "blocked by hook") {
		t.Errorf("expected 'blocked by hook' in content, got: %q", results[0].Content)
	}
	if mockT.execCount != 0 {
		t.Error("tool should not have been executed when hook blocks")
	}
}

func TestExecuteTools_ToolError(t *testing.T) {
	mockT := &configurableMockTool{
		name:    "Bash",
		execErr: fmt.Errorf("command failed: exit status 1"),
	}
	registry := tools.NewRegistry()
	registry.Register(mockT)

	ctx := context.Background()
	out := make(chan Message, 64)
	calls := []toolCall{makeToolCall("tc1", "Bash", map[string]any{"command": "false"})}
	sessionAllow := map[string]bool{"Bash": true}

	results, err := executeTools(ctx, out, registry, calls, sessionAllow, nil, nil)
	close(out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].IsError {
		t.Error("expected error result")
	}
	if !strings.Contains(results[0].Content, "tool execution error") {
		t.Errorf("expected 'tool execution error' in content, got: %q", results[0].Content)
	}
}

func TestExecuteTools_Success(t *testing.T) {
	mockT := &configurableMockTool{
		name:   "Read",
		result: tools.Result{Output: "file contents here"},
	}
	registry := tools.NewRegistry()
	registry.Register(mockT)

	ctx := context.Background()
	out := make(chan Message, 64)
	calls := []toolCall{makeToolCall("tc1", "Read", map[string]any{"file_path": "/tmp/test"})}

	// Read is in alwaysAllowTools, so no permission needed.
	results, err := executeTools(ctx, out, registry, calls, map[string]bool{}, nil, nil)
	close(out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].IsError {
		t.Error("expected non-error result")
	}
	if results[0].Content != "file contents here" {
		t.Errorf("unexpected content: %q", results[0].Content)
	}
	if mockT.execCount != 1 {
		t.Errorf("expected 1 execution, got %d", mockT.execCount)
	}

	// Verify emitted messages.
	var gotCall, gotResult bool
	for m := range out {
		if m.Type == MessageToolCall && m.ToolName == "Read" {
			gotCall = true
		}
		if m.Type == MessageToolResult && m.ToolOutput == "file contents here" {
			gotResult = true
		}
	}
	if !gotCall {
		t.Error("expected MessageToolCall")
	}
	if !gotResult {
		t.Error("expected MessageToolResult")
	}
}

func TestExecuteTools_MultipleTools(t *testing.T) {
	readTool := &configurableMockTool{
		name:   "Read",
		result: tools.Result{Output: "read output"},
	}
	grepTool := &configurableMockTool{
		name:   "Grep",
		result: tools.Result{Output: "grep output"},
	}
	registry := tools.NewRegistry()
	registry.Register(readTool)
	registry.Register(grepTool)

	ctx := context.Background()
	out := make(chan Message, 64)
	calls := []toolCall{
		makeToolCall("tc1", "Read", map[string]any{"file_path": "/tmp/a"}),
		makeToolCall("tc2", "Grep", map[string]any{"pattern": "test"}),
	}

	results, err := executeTools(ctx, out, registry, calls, map[string]bool{}, nil, nil)
	close(out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Content != "read output" {
		t.Errorf("result[0] content = %q, want 'read output'", results[0].Content)
	}
	if results[1].Content != "grep output" {
		t.Errorf("result[1] content = %q, want 'grep output'", results[1].Content)
	}
}

func TestExecuteTools_UserDeniesPermission(t *testing.T) {
	mockT := &configurableMockTool{
		name:   "Bash",
		result: tools.Result{Output: "should not run"},
	}
	registry := tools.NewRegistry()
	registry.Register(mockT)

	ctx := context.Background()
	out := make(chan Message, 64)
	calls := []toolCall{makeToolCall("tc1", "Bash", map[string]any{"command": "rm -rf /"})}

	// No permissions config, no session allow -- will trigger permission request.
	// Drain output in a goroutine and respond to permission requests.
	go func() {
		for m := range out {
			if m.Type == MessagePermissionRequest && m.PermissionResponse != nil {
				m.PermissionResponse <- PermissionDeny
			}
		}
	}()

	results, err := executeTools(ctx, out, registry, calls, map[string]bool{}, nil, nil)
	close(out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].IsError {
		t.Error("expected error result for denied permission")
	}
	if results[0].Content != "Permission denied by user." {
		t.Errorf("unexpected content: %q", results[0].Content)
	}
	if mockT.execCount != 0 {
		t.Error("tool should not have been executed")
	}
}

func TestExecuteTools_UserAlwaysApprove(t *testing.T) {
	mockT := &configurableMockTool{
		name:   "Bash",
		result: tools.Result{Output: "executed"},
	}
	registry := tools.NewRegistry()
	registry.Register(mockT)

	ctx := context.Background()
	out := make(chan Message, 64)
	sessionAllow := map[string]bool{}

	// Two calls to the same tool -- first will prompt, second should skip prompt.
	calls := []toolCall{
		makeToolCall("tc1", "Bash", map[string]any{"command": "echo 1"}),
		makeToolCall("tc2", "Bash", map[string]any{"command": "echo 2"}),
	}

	permRequestCount := 0
	go func() {
		for m := range out {
			if m.Type == MessagePermissionRequest && m.PermissionResponse != nil {
				permRequestCount++
				m.PermissionResponse <- PermissionAlwaysApprove
			}
		}
	}()

	results, err := executeTools(ctx, out, registry, calls, sessionAllow, nil, nil)
	close(out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Only one permission request should have been made (AlwaysApprove adds to sessionAllow).
	if permRequestCount != 1 {
		t.Errorf("expected 1 permission request, got %d", permRequestCount)
	}
	// sessionAllow should now include Bash.
	if !sessionAllow["Bash"] {
		t.Error("expected Bash in sessionAllow after AlwaysApprove")
	}
}

func TestExecuteTools_ContextCancelled(t *testing.T) {
	mockT := &configurableMockTool{
		name:   "Read",
		result: tools.Result{Output: "ok"},
	}
	registry := tools.NewRegistry()
	registry.Register(mockT)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	out := make(chan Message, 64)
	calls := []toolCall{makeToolCall("tc1", "Read", map[string]any{})}

	_, err := executeTools(ctx, out, registry, calls, map[string]bool{}, nil, nil)
	close(out)

	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !strings.Contains(err.Error(), "context cancelled") {
		t.Errorf("expected 'context cancelled' in error, got: %v", err)
	}
}

// --- Tests for processTurn ---

func TestProcessTurn_TextResponse(t *testing.T) {
	events := make(chan api.StreamEvent, 20)

	// Send a simple text response stream.
	events <- api.StreamEvent{
		Type: "message_start",
		Message: &api.MessageResponse{
			Usage: api.Usage{InputTokens: 100, OutputTokens: 0},
		},
	}
	events <- api.StreamEvent{
		Type:         "content_block_start",
		Index:        0,
		ContentBlock: &api.ContentBlock{Type: "text", Text: ""},
	}
	events <- api.StreamEvent{
		Type:  "content_block_delta",
		Index: 0,
		Delta: &api.Delta{Type: "text_delta", Text: "Hello "},
	}
	events <- api.StreamEvent{
		Type:  "content_block_delta",
		Index: 0,
		Delta: &api.Delta{Type: "text_delta", Text: "World"},
	}
	events <- api.StreamEvent{
		Type:  "content_block_stop",
		Index: 0,
	}
	events <- api.StreamEvent{
		Type:       "message_delta",
		StopReason: "end_turn",
		Usage:      &api.Usage{OutputTokens: 50},
	}
	events <- api.StreamEvent{Type: "message_stop"}
	close(events)

	out := make(chan Message, 64)
	ctx := context.Background()

	result, err := processTurn(ctx, events, out)
	close(out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.stopReason != "end_turn" {
		t.Errorf("stopReason = %q, want 'end_turn'", result.stopReason)
	}
	if result.inputTokens != 100 {
		t.Errorf("inputTokens = %d, want 100", result.inputTokens)
	}
	if result.outputTokens != 50 {
		t.Errorf("outputTokens = %d, want 50", result.outputTokens)
	}
	if len(result.contentBlocks) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(result.contentBlocks))
	}
	if result.contentBlocks[0].Text != "Hello World" {
		t.Errorf("text = %q, want 'Hello World'", result.contentBlocks[0].Text)
	}
	if len(result.toolCalls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(result.toolCalls))
	}

	// Verify emitted text deltas.
	var collected string
	for m := range out {
		if m.Type == MessageTextDelta {
			collected += m.Text
		}
	}
	if collected != "Hello World" {
		t.Errorf("emitted text = %q, want 'Hello World'", collected)
	}
}

func TestProcessTurn_ToolUseResponse(t *testing.T) {
	events := make(chan api.StreamEvent, 20)

	events <- api.StreamEvent{
		Type: "message_start",
		Message: &api.MessageResponse{
			Usage: api.Usage{InputTokens: 200, OutputTokens: 0},
		},
	}
	events <- api.StreamEvent{
		Type:  "content_block_start",
		Index: 0,
		ContentBlock: &api.ContentBlock{
			Type: "tool_use",
			ID:   "tool_123",
			Name: "Read",
		},
	}
	events <- api.StreamEvent{
		Type:  "content_block_delta",
		Index: 0,
		Delta: &api.Delta{Type: "input_json_delta", PartialJSON: `{"file_pa`},
	}
	events <- api.StreamEvent{
		Type:  "content_block_delta",
		Index: 0,
		Delta: &api.Delta{Type: "input_json_delta", PartialJSON: `th":"/tmp/test"}`},
	}
	events <- api.StreamEvent{
		Type:  "content_block_stop",
		Index: 0,
	}
	events <- api.StreamEvent{
		Type:       "message_delta",
		StopReason: "tool_use",
		Usage:      &api.Usage{OutputTokens: 30},
	}
	events <- api.StreamEvent{Type: "message_stop"}
	close(events)

	out := make(chan Message, 64)
	ctx := context.Background()

	result, err := processTurn(ctx, events, out)
	close(out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.stopReason != "tool_use" {
		t.Errorf("stopReason = %q, want 'tool_use'", result.stopReason)
	}
	if len(result.toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.toolCalls))
	}
	tc := result.toolCalls[0]
	if tc.id != "tool_123" {
		t.Errorf("tool call id = %q, want 'tool_123'", tc.id)
	}
	if tc.name != "Read" {
		t.Errorf("tool call name = %q, want 'Read'", tc.name)
	}

	var parsed map[string]string
	if err := json.Unmarshal(tc.input, &parsed); err != nil {
		t.Fatalf("failed to parse tool input: %v", err)
	}
	if parsed["file_path"] != "/tmp/test" {
		t.Errorf("file_path = %q, want '/tmp/test'", parsed["file_path"])
	}
}

func TestProcessTurn_ThinkingDelta(t *testing.T) {
	events := make(chan api.StreamEvent, 20)

	events <- api.StreamEvent{
		Type:    "message_start",
		Message: &api.MessageResponse{Usage: api.Usage{InputTokens: 50}},
	}
	events <- api.StreamEvent{
		Type:         "content_block_start",
		Index:        0,
		ContentBlock: &api.ContentBlock{Type: "thinking"},
	}
	events <- api.StreamEvent{
		Type:  "content_block_delta",
		Index: 0,
		Delta: &api.Delta{Type: "thinking_delta", Thinking: "Let me think..."},
	}
	events <- api.StreamEvent{Type: "content_block_stop", Index: 0}
	events <- api.StreamEvent{
		Type:         "content_block_start",
		Index:        1,
		ContentBlock: &api.ContentBlock{Type: "text"},
	}
	events <- api.StreamEvent{
		Type:  "content_block_delta",
		Index: 1,
		Delta: &api.Delta{Type: "text_delta", Text: "Answer"},
	}
	events <- api.StreamEvent{Type: "content_block_stop", Index: 1}
	events <- api.StreamEvent{
		Type:       "message_delta",
		StopReason: "end_turn",
		Usage:      &api.Usage{OutputTokens: 40},
	}
	events <- api.StreamEvent{Type: "message_stop"}
	close(events)

	out := make(chan Message, 64)
	result, err := processTurn(context.Background(), events, out)
	close(out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.contentBlocks) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(result.contentBlocks))
	}
	if result.contentBlocks[0].Thinking != "Let me think..." {
		t.Errorf("thinking = %q, want 'Let me think...'", result.contentBlocks[0].Thinking)
	}
	if result.contentBlocks[1].Text != "Answer" {
		t.Errorf("text = %q, want 'Answer'", result.contentBlocks[1].Text)
	}

	// Verify thinking and text deltas were emitted.
	var gotThinking, gotText bool
	for m := range out {
		if m.Type == MessageThinkingDelta {
			gotThinking = true
		}
		if m.Type == MessageTextDelta {
			gotText = true
		}
	}
	if !gotThinking {
		t.Error("expected thinking delta emission")
	}
	if !gotText {
		t.Error("expected text delta emission")
	}
}

func TestProcessTurn_StreamError(t *testing.T) {
	events := make(chan api.StreamEvent, 5)

	events <- api.StreamEvent{
		Type:    "message_start",
		Message: &api.MessageResponse{Usage: api.Usage{InputTokens: 10}},
	}
	events <- api.StreamEvent{
		Type: "error",
		Err:  fmt.Errorf("rate limit exceeded"),
	}
	close(events)

	out := make(chan Message, 64)
	_, err := processTurn(context.Background(), events, out)
	close(out)

	if err == nil {
		t.Fatal("expected error from stream error event")
	}
	if !strings.Contains(err.Error(), "stream error") {
		t.Errorf("expected 'stream error' in error, got: %v", err)
	}
}

func TestProcessTurn_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	events := make(chan api.StreamEvent, 5)
	events <- api.StreamEvent{
		Type:    "message_start",
		Message: &api.MessageResponse{Usage: api.Usage{InputTokens: 10}},
	}

	// Cancel before sending more events.
	cancel()

	events <- api.StreamEvent{
		Type:         "content_block_start",
		Index:        0,
		ContentBlock: &api.ContentBlock{Type: "text"},
	}
	close(events)

	out := make(chan Message, 64)
	_, err := processTurn(ctx, events, out)
	close(out)

	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !strings.Contains(err.Error(), "context cancelled") {
		t.Errorf("expected 'context cancelled' in error, got: %v", err)
	}
}

func TestProcessTurn_EmptyToolUseInput(t *testing.T) {
	events := make(chan api.StreamEvent, 20)

	events <- api.StreamEvent{
		Type:    "message_start",
		Message: &api.MessageResponse{Usage: api.Usage{InputTokens: 50}},
	}
	events <- api.StreamEvent{
		Type:  "content_block_start",
		Index: 0,
		ContentBlock: &api.ContentBlock{
			Type: "tool_use",
			ID:   "tool_empty",
			Name: "Glob",
		},
	}
	// No input_json_delta events -- empty input should default to "{}".
	events <- api.StreamEvent{Type: "content_block_stop", Index: 0}
	events <- api.StreamEvent{
		Type:       "message_delta",
		StopReason: "tool_use",
		Usage:      &api.Usage{OutputTokens: 10},
	}
	events <- api.StreamEvent{Type: "message_stop"}
	close(events)

	out := make(chan Message, 64)
	result, err := processTurn(context.Background(), events, out)
	close(out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.toolCalls))
	}
	if string(result.toolCalls[0].input) != "{}" {
		t.Errorf("expected empty input '{}', got %q", string(result.toolCalls[0].input))
	}
}

func TestProcessTurn_CacheTokens(t *testing.T) {
	events := make(chan api.StreamEvent, 10)

	events <- api.StreamEvent{
		Type: "message_start",
		Message: &api.MessageResponse{
			Usage: api.Usage{
				InputTokens:              100,
				CacheCreationInputTokens: 50,
				CacheReadInputTokens:     25,
			},
		},
	}
	events <- api.StreamEvent{
		Type:         "content_block_start",
		Index:        0,
		ContentBlock: &api.ContentBlock{Type: "text"},
	}
	events <- api.StreamEvent{
		Type:  "content_block_delta",
		Index: 0,
		Delta: &api.Delta{Type: "text_delta", Text: "Hi"},
	}
	events <- api.StreamEvent{Type: "content_block_stop", Index: 0}
	events <- api.StreamEvent{
		Type:       "message_delta",
		StopReason: "end_turn",
		Usage: &api.Usage{
			OutputTokens:             30,
			CacheCreationInputTokens: 10,
			CacheReadInputTokens:     5,
		},
	}
	events <- api.StreamEvent{Type: "message_stop"}
	close(events)

	out := make(chan Message, 64)
	result, err := processTurn(context.Background(), events, out)
	close(out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.cacheCreationTokens != 60 {
		t.Errorf("cacheCreationTokens = %d, want 60", result.cacheCreationTokens)
	}
	if result.cacheReadTokens != 30 {
		t.Errorf("cacheReadTokens = %d, want 30", result.cacheReadTokens)
	}
	if result.lastRequestInputTokens != 100 {
		t.Errorf("lastRequestInputTokens = %d, want 100", result.lastRequestInputTokens)
	}
}

func TestProcessTurn_NilContentBlockStart(t *testing.T) {
	events := make(chan api.StreamEvent, 10)

	events <- api.StreamEvent{
		Type:    "message_start",
		Message: &api.MessageResponse{Usage: api.Usage{InputTokens: 10}},
	}
	// content_block_start with nil ContentBlock should be skipped.
	events <- api.StreamEvent{
		Type:         "content_block_start",
		Index:        0,
		ContentBlock: nil,
	}
	events <- api.StreamEvent{
		Type:       "message_delta",
		StopReason: "end_turn",
		Usage:      &api.Usage{OutputTokens: 5},
	}
	events <- api.StreamEvent{Type: "message_stop"}
	close(events)

	out := make(chan Message, 64)
	result, err := processTurn(context.Background(), events, out)
	close(out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.contentBlocks) != 0 {
		t.Errorf("expected 0 content blocks, got %d", len(result.contentBlocks))
	}
}

func TestProcessTurn_NilDelta(t *testing.T) {
	events := make(chan api.StreamEvent, 10)

	events <- api.StreamEvent{
		Type:    "message_start",
		Message: &api.MessageResponse{Usage: api.Usage{InputTokens: 10}},
	}
	events <- api.StreamEvent{
		Type:         "content_block_start",
		Index:        0,
		ContentBlock: &api.ContentBlock{Type: "text"},
	}
	// content_block_delta with nil Delta should be skipped.
	events <- api.StreamEvent{
		Type:  "content_block_delta",
		Index: 0,
		Delta: nil,
	}
	events <- api.StreamEvent{Type: "content_block_stop", Index: 0}
	events <- api.StreamEvent{
		Type:       "message_delta",
		StopReason: "end_turn",
		Usage:      &api.Usage{OutputTokens: 5},
	}
	events <- api.StreamEvent{Type: "message_stop"}
	close(events)

	out := make(chan Message, 64)
	result, err := processTurn(context.Background(), events, out)
	close(out)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.contentBlocks[0].Text != "" {
		t.Errorf("expected empty text, got %q", result.contentBlocks[0].Text)
	}
}

// --- Test for emit with cancelled context ---

func TestEmit_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Use an unbuffered channel -- emit should not block because ctx is done.
	out := make(chan Message)
	emit(ctx, out, Message{Type: MessageTextDelta, Text: "should not block"})
	// If we get here without blocking, the test passes.
}

// --- Test for runTurn integration with tool_use ---

func TestRunTurn_ToolUseIntegration(t *testing.T) {
	// Build an SSE response that includes a tool_use block, followed by
	// a second response (after tool execution) that ends.
	var toolUseResp strings.Builder
	toolUseResp.WriteString("event: message_start\n")
	toolUseResp.WriteString(`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"usage":{"input_tokens":100,"output_tokens":0}}}`)
	toolUseResp.WriteString("\n\n")

	toolUseResp.WriteString("event: content_block_start\n")
	toolUseResp.WriteString(`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tu_1","name":"Read"}}`)
	toolUseResp.WriteString("\n\n")

	toolUseResp.WriteString("event: content_block_delta\n")
	toolUseResp.WriteString(`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"file_path\":\"/tmp/test\"}"}}`)
	toolUseResp.WriteString("\n\n")

	toolUseResp.WriteString("event: content_block_stop\n")
	toolUseResp.WriteString(`data: {"type":"content_block_stop","index":0}`)
	toolUseResp.WriteString("\n\n")

	toolUseResp.WriteString("event: message_delta\n")
	toolUseResp.WriteString(`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":20}}`)
	toolUseResp.WriteString("\n\n")

	toolUseResp.WriteString("event: message_stop\n")
	toolUseResp.WriteString(`data: {"type":"message_stop"}`)
	toolUseResp.WriteString("\n\n")

	srv := newMockServer(
		toolUseResp.String(),
		buildSSEResponse("Done!", "end_turn"),
	)
	defer srv.Close()

	readTool := &configurableMockTool{
		name:   "Read",
		result: tools.Result{Output: "test file contents"},
	}

	cfg := Config{
		Model: "test-model",
		Creds: mockCreds{},
		Tools: []tools.Tool{readTool},
	}
	s := NewSession(cfg)
	s.client.SetEndpoint(srv.URL)

	msgs := drainMessages(s.Turn(context.Background(), "Read /tmp/test"))

	// Verify we got tool call, tool result, text, and done messages.
	var gotToolCall, gotToolResult, gotText, gotDone bool
	for _, m := range msgs {
		switch m.Type {
		case MessageToolCall:
			if m.ToolName == "Read" {
				gotToolCall = true
			}
		case MessageToolResult:
			if m.ToolOutput == "test file contents" {
				gotToolResult = true
			}
		case MessageTextDelta:
			if m.Text == "Done!" {
				gotText = true
			}
		case MessageDone:
			gotDone = true
		}
	}

	if !gotToolCall {
		t.Error("expected tool call message for Read")
	}
	if !gotToolResult {
		t.Error("expected tool result message")
	}
	if !gotText {
		t.Error("expected text delta 'Done!'")
	}
	if !gotDone {
		t.Error("expected done message")
	}
	if readTool.execCount != 1 {
		t.Errorf("expected Read to execute once, got %d", readTool.execCount)
	}
}

func TestRunTurn_MaxTurnsReached(t *testing.T) {
	// Create a tool_use response that keeps looping.
	var toolUseResp strings.Builder
	toolUseResp.WriteString("event: message_start\n")
	toolUseResp.WriteString(`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"usage":{"input_tokens":50,"output_tokens":0}}}`)
	toolUseResp.WriteString("\n\n")
	toolUseResp.WriteString("event: content_block_start\n")
	toolUseResp.WriteString(`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tu_1","name":"Read"}}`)
	toolUseResp.WriteString("\n\n")
	toolUseResp.WriteString("event: content_block_delta\n")
	toolUseResp.WriteString(`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{}"}}`)
	toolUseResp.WriteString("\n\n")
	toolUseResp.WriteString("event: content_block_stop\n")
	toolUseResp.WriteString(`data: {"type":"content_block_stop","index":0}`)
	toolUseResp.WriteString("\n\n")
	toolUseResp.WriteString("event: message_delta\n")
	toolUseResp.WriteString(`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":10}}`)
	toolUseResp.WriteString("\n\n")
	toolUseResp.WriteString("event: message_stop\n")
	toolUseResp.WriteString(`data: {"type":"message_stop"}`)
	toolUseResp.WriteString("\n\n")

	// Server always returns tool_use.
	srv := newMockServer(toolUseResp.String())
	defer srv.Close()

	maxTurns := 2
	cfg := Config{
		Model:    "test-model",
		Creds:    mockCreds{},
		Tools:    []tools.Tool{&configurableMockTool{name: "Read", result: tools.Result{Output: "ok"}}},
		MaxTurns: &maxTurns,
	}
	s := NewSession(cfg)
	s.client.SetEndpoint(srv.URL)

	msgs := drainMessages(s.Turn(context.Background(), "loop"))

	var gotMaxTurns bool
	for _, m := range msgs {
		if m.Type == MessageMaxTurnsReached {
			gotMaxTurns = true
		}
	}
	if !gotMaxTurns {
		t.Error("expected MessageMaxTurnsReached")
	}
}

func TestRunTurn_MaxTokensContinuation(t *testing.T) {
	// First response hits max_tokens, second completes normally.
	maxTokensResp := buildSSEResponse("partial...", "max_tokens")

	srv := newMockServer(
		maxTokensResp,
		buildSSEResponse("complete", "end_turn"),
	)
	defer srv.Close()

	s := newTestSession(srv.URL)
	msgs := drainMessages(s.Turn(context.Background(), "test"))

	var gotDone bool
	var texts []string
	for _, m := range msgs {
		if m.Type == MessageTextDelta {
			texts = append(texts, m.Text)
		}
		if m.Type == MessageDone {
			gotDone = true
		}
	}
	if !gotDone {
		t.Error("expected done message after continuation")
	}
	if len(texts) < 2 {
		t.Errorf("expected at least 2 text deltas, got %d", len(texts))
	}
}

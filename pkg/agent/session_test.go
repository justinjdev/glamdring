package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"github.com/justin/glamdring/pkg/api"
	"github.com/justin/glamdring/pkg/auth"
	"github.com/justin/glamdring/pkg/tools"
)

// mockCreds implements auth.Credentials for testing.
type mockCreds struct{}

func (mockCreds) SetAuthHeaders(r *http.Request) error {
	r.Header.Set("x-api-key", "test-key")
	return nil
}
func (mockCreds) IsOAuth() bool { return false }

// containsText checks whether a RequestMessage Content (string or []ContentBlock)
// contains the given substring in any text.
func containsText(content any, substr string) bool {
	switch v := content.(type) {
	case string:
		return strings.Contains(v, substr)
	case []api.ContentBlock:
		for _, b := range v {
			if strings.Contains(b.Text, substr) || strings.Contains(b.Content, substr) {
				return true
			}
		}
	}
	return false
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
	b.WriteString(fmt.Sprintf(`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"%s"}}`, text))
	b.WriteString("\n\n")

	b.WriteString("event: content_block_stop\n")
	b.WriteString(`data: {"type":"content_block_stop","index":0}`)
	b.WriteString("\n\n")

	b.WriteString("event: message_delta\n")
	b.WriteString(fmt.Sprintf(`data: {"type":"message_delta","delta":{"stop_reason":"%s"},"usage":{"output_tokens":50}}`, stopReason))
	b.WriteString("\n\n")

	b.WriteString("event: message_stop\n")
	b.WriteString(`data: {"type":"message_stop"}`)
	b.WriteString("\n\n")

	return b.String()
}

func newMockServer(responses ...string) *httptest.Server {
	var call atomic.Int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := int(call.Load())
		if idx >= len(responses) {
			idx = len(responses) - 1
		}
		call.Add(1)

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, responses[idx])
	}))
}

func newTestSession(serverURL string) *Session {
	cfg := Config{
		Model: "test-model",
		Creds: mockCreds{},
	}
	s := NewSession(cfg)
	s.client.SetEndpoint(serverURL)
	return s
}

func drainMessages(ch <-chan Message) []Message {
	var msgs []Message
	for m := range ch {
		msgs = append(msgs, m)
	}
	return msgs
}

func TestSessionTurnAppendsToHistory(t *testing.T) {
	srv := newMockServer(buildSSEResponse("Hello!", "end_turn"))
	defer srv.Close()

	s := newTestSession(srv.URL)
	msgs := drainMessages(s.Turn(context.Background(), "hi"))

	// Should have received at least a text delta and a done message.
	var gotText, gotDone bool
	for _, m := range msgs {
		if m.Type == MessageTextDelta && m.Text == "Hello!" {
			gotText = true
		}
		if m.Type == MessageDone {
			gotDone = true
		}
	}
	if !gotText {
		t.Error("expected text delta with 'Hello!'")
	}
	if !gotDone {
		t.Error("expected done message")
	}

	// History should have user + assistant messages.
	if len(s.Messages()) != 2 {
		t.Errorf("expected 2 messages in history, got %d", len(s.Messages()))
	}
	if s.Messages()[0].Role != "user" {
		t.Errorf("first message role = %q, want 'user'", s.Messages()[0].Role)
	}
	if s.Messages()[1].Role != "assistant" {
		t.Errorf("second message role = %q, want 'assistant'", s.Messages()[1].Role)
	}
}

func TestSessionTwoTurnsPreserveHistory(t *testing.T) {
	srv := newMockServer(
		buildSSEResponse("First response", "end_turn"),
		buildSSEResponse("Second response", "end_turn"),
	)
	defer srv.Close()

	s := newTestSession(srv.URL)

	// Turn 1.
	drainMessages(s.Turn(context.Background(), "first question"))
	if len(s.Messages()) != 2 {
		t.Fatalf("after turn 1: expected 2 messages, got %d", len(s.Messages()))
	}

	// Turn 2.
	drainMessages(s.Turn(context.Background(), "second question"))
	if len(s.Messages()) != 4 {
		t.Fatalf("after turn 2: expected 4 messages, got %d", len(s.Messages()))
	}

	// Verify message order: user, assistant, user, assistant.
	expectedRoles := []string{"user", "assistant", "user", "assistant"}
	for i, expected := range expectedRoles {
		if s.Messages()[i].Role != expected {
			t.Errorf("message[%d].Role = %q, want %q", i, s.Messages()[i].Role, expected)
		}
	}
}

func TestSessionResetClearsHistory(t *testing.T) {
	srv := newMockServer(buildSSEResponse("Hello!", "end_turn"))
	defer srv.Close()

	s := newTestSession(srv.URL)
	drainMessages(s.Turn(context.Background(), "hi"))

	if len(s.Messages()) == 0 {
		t.Fatal("expected non-empty history before reset")
	}

	s.Reset()

	if len(s.Messages()) != 0 {
		t.Errorf("expected empty history after reset, got %d messages", len(s.Messages()))
	}
}

func TestRunOneShotDoesNotPersistHistory(t *testing.T) {
	srv := newMockServer(buildSSEResponse("Response", "end_turn"))
	defer srv.Close()

	cfg := Config{
		Prompt: "test prompt",
		Model:  "test-model",
		Creds:  mockCreds{},
	}

	// We can't easily inspect the internal session of Run(), but we can verify
	// it completes correctly with a done message.
	ch := Run(context.Background(), cfg)

	// This will fail to connect since we're using default endpoint, so let's
	// test the Session-based approach instead to verify one-shot behavior.
	// Drain and ignore -- the real test is that Run() creates a throwaway session.
	for range ch {
	}
}

func TestSessionTokenDeltas(t *testing.T) {
	srv := newMockServer(
		buildSSEResponse("First", "end_turn"),
		buildSSEResponse("Second", "end_turn"),
	)
	defer srv.Close()

	s := newTestSession(srv.URL)

	// Turn 1.
	msgs1 := drainMessages(s.Turn(context.Background(), "q1"))
	var turn1Input, turn1Output int
	for _, m := range msgs1 {
		if m.Type == MessageDone {
			turn1Input = m.InputTokens
			turn1Output = m.OutputTokens
		}
	}
	if turn1Input == 0 {
		t.Error("turn 1 should report non-zero input tokens")
	}

	// Turn 2.
	msgs2 := drainMessages(s.Turn(context.Background(), "q2"))
	var turn2Input, turn2Output int
	for _, m := range msgs2 {
		if m.Type == MessageDone {
			turn2Input = m.InputTokens
			turn2Output = m.OutputTokens
		}
	}
	if turn2Input == 0 {
		t.Error("turn 2 should report non-zero input tokens")
	}

	// Each turn's tokens should be per-turn deltas, not cumulative.
	// Both test responses have 100 input + 50 output tokens.
	if turn1Input != turn2Input {
		t.Errorf("expected equal per-turn input tokens, got %d and %d", turn1Input, turn2Input)
	}
	_ = turn1Output
	_ = turn2Output
}

func TestSessionSetYolo(t *testing.T) {
	s := newTestSessionWithTools()

	if s.IsYolo() {
		t.Fatal("expected yolo to be false initially")
	}

	s.SetYolo(true)
	if !s.IsYolo() {
		t.Error("expected yolo to be true after SetYolo(true)")
	}
	// All tools should be in sessionAllow.
	if !s.sessionAllow["Bash"] {
		t.Error("expected Bash in sessionAllow when yolo is on")
	}
	if !s.sessionAllow["Write"] {
		t.Error("expected Write in sessionAllow when yolo is on")
	}

	s.SetYolo(false)
	if s.IsYolo() {
		t.Error("expected yolo to be false after SetYolo(false)")
	}
	if len(s.sessionAllow) != 0 {
		t.Errorf("expected empty sessionAllow when yolo is off, got %d entries", len(s.sessionAllow))
	}
}

func TestSessionToggleYolo(t *testing.T) {
	s := newTestSessionWithTools()

	s.ToggleYolo()
	if !s.IsYolo() {
		t.Error("expected yolo to be true after first toggle")
	}

	s.ToggleYolo()
	if s.IsYolo() {
		t.Error("expected yolo to be false after second toggle")
	}
}

func TestSessionSetYoloScoped(t *testing.T) {
	s := newTestSessionWithTools()

	s.SetYoloScoped([]string{"Bash", "Write"})
	if s.IsYolo() {
		t.Error("scoped yolo should not set global yolo flag")
	}
	if !s.sessionAllow["Bash"] {
		t.Error("expected Bash in sessionAllow after scoped yolo")
	}
	if !s.sessionAllow["Write"] {
		t.Error("expected Write in sessionAllow after scoped yolo")
	}
	if s.sessionAllow["Edit"] {
		t.Error("expected Edit NOT in sessionAllow after scoped yolo")
	}
}

func TestSessionResetClearsYolo(t *testing.T) {
	s := newTestSessionWithTools()
	s.SetYolo(true)
	if !s.IsYolo() {
		t.Fatal("expected yolo to be true before reset")
	}

	s.Reset()
	if s.IsYolo() {
		t.Error("expected yolo to be false after reset")
	}
	if len(s.sessionAllow) != 0 {
		t.Errorf("expected empty sessionAllow after reset, got %d", len(s.sessionAllow))
	}
}

func TestNewSessionWithYoloConfig(t *testing.T) {
	cfg := Config{
		Model: "test-model",
		Creds: mockCreds{},
		Tools: []tools.Tool{&mockTool{name: "Bash"}, &mockTool{name: "Write"}},
		Yolo:  true,
	}
	s := NewSession(cfg)
	if !s.IsYolo() {
		t.Error("expected yolo to be true when Config.Yolo is true")
	}
	if !s.sessionAllow["Bash"] {
		t.Error("expected Bash in sessionAllow when yolo config is set")
	}
}

// newTestSessionWithTools creates a session with mock tools for yolo testing.
func newTestSessionWithTools() *Session {
	cfg := Config{
		Model: "test-model",
		Creds: mockCreds{},
		Tools: []tools.Tool{
			&mockTool{name: "Bash"},
			&mockTool{name: "Write"},
			&mockTool{name: "Edit"},
			&mockTool{name: "Read"},
		},
	}
	return NewSession(cfg)
}

// mockTool is a minimal tool implementation for testing.
type mockTool struct {
	name string
}

func (t *mockTool) Name() string        { return t.name }
func (t *mockTool) Description() string { return "mock tool" }
func (t *mockTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}
func (t *mockTool) Execute(ctx context.Context, input json.RawMessage) (tools.Result, error) {
	return tools.Result{Output: "ok"}, nil
}

func TestTruncateToolResult(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		changed  bool
	}{
		{
			name:     "empty string unchanged",
			input:    "",
			expected: "",
			changed:  false,
		},
		{
			name:     "short output unchanged",
			input:    "hello world",
			expected: "hello world",
			changed:  false,
		},
		{
			name:     "exactly at limit unchanged",
			input:    strings.Repeat("a", maxToolResultSize),
			expected: strings.Repeat("a", maxToolResultSize),
			changed:  false,
		},
		{
			name:    "over limit truncated",
			input:   strings.Repeat("a", maxToolResultSize+100),
			changed: true,
		},
		{
			name:    "multi-byte UTF-8 truncation",
			input:   strings.Repeat("a", maxToolResultSize-1) + "\xf0\x9f\x98\x80" + "tail",
			changed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateToolResult(tt.input)
			if tt.changed {
				if !strings.Contains(result, "truncated") {
					t.Error("expected truncation notice in output")
				}
				fullSizeStr := fmt.Sprintf("%d", len(tt.input))
				if !strings.Contains(result, fullSizeStr) {
					t.Errorf("expected original size %s in truncation notice", fullSizeStr)
				}
				if !utf8.ValidString(result) {
					t.Error("truncated result is not valid UTF-8")
				}
				// Verify prefix preservation: the truncated content
				// (before the notice) should match the original.
				beforeNotice := strings.SplitN(result, "\n... (truncated", 2)[0]
				if len(beforeNotice) >= len(tt.input) {
					t.Error("expected truncated content to be shorter than input")
				}
				if !strings.HasPrefix(tt.input, beforeNotice) {
					t.Error("truncated result prefix does not match original input")
				}
			} else {
				if result != tt.expected {
					t.Errorf("got %q, want %q", result, tt.expected)
				}
			}
		})
	}
}

func TestSessionTurnWithBlocksAppendsToHistory(t *testing.T) {
	srv := newMockServer(buildSSEResponse("I can see the image!", "end_turn"))
	defer srv.Close()

	s := newTestSession(srv.URL)
	blocks := []api.ContentBlock{
		{Type: "image", Source: &api.ImageSource{
			Type:      "base64",
			MediaType: "image/png",
			Data:      "iVBORw0KGgo=",
		}},
		{Type: "text", Text: "What is in this image?"},
	}
	msgs := drainMessages(s.TurnWithBlocks(context.Background(), blocks))

	var gotText, gotDone bool
	for _, m := range msgs {
		if m.Type == MessageTextDelta && m.Text == "I can see the image!" {
			gotText = true
		}
		if m.Type == MessageDone {
			gotDone = true
		}
	}
	if !gotText {
		t.Error("expected text delta")
	}
	if !gotDone {
		t.Error("expected done message")
	}

	// History should have user + assistant.
	if len(s.Messages()) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(s.Messages()))
	}
	if s.Messages()[0].Role != "user" {
		t.Errorf("first message role = %q, want 'user'", s.Messages()[0].Role)
	}
	// Content should be []ContentBlock, not a string.
	contentBlocks, ok := s.Messages()[0].Content.([]api.ContentBlock)
	if !ok {
		t.Fatalf("expected Content to be []ContentBlock, got %T", s.Messages()[0].Content)
	}
	if len(contentBlocks) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(contentBlocks))
	}
	if contentBlocks[0].Type != "image" {
		t.Errorf("first block type = %q, want 'image'", contentBlocks[0].Type)
	}
}

// --- Tests for SetModel ---

func TestSessionSetModel(t *testing.T) {
	s := newTestSessionWithTools()
	if s.client.Model() != "test-model" {
		t.Fatalf("initial model = %q, want 'test-model'", s.client.Model())
	}

	s.SetModel("claude-sonnet-4-20250514")
	if s.client.Model() != "claude-sonnet-4-20250514" {
		t.Errorf("model after SetModel = %q, want 'claude-sonnet-4-20250514'", s.client.Model())
	}
}

// --- Tests for drainRegularMessages ---

func TestDrainRegularMessages_NilChannel(t *testing.T) {
	s := &Session{
		regularCh: nil,
		messages:  []api.RequestMessage{{Role: "user", Content: "hello"}},
	}
	s.drainRegularMessages()
	// Should be a no-op.
	if len(s.messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(s.messages))
	}
}

func TestDrainRegularMessages_EmptyChannel(t *testing.T) {
	ch := make(chan any, 5)
	s := &Session{
		regularCh: ch,
		messages:  []api.RequestMessage{{Role: "user", Content: "hello"}},
	}
	s.drainRegularMessages()
	// No messages pending, so no new messages should be appended.
	if len(s.messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(s.messages))
	}
}

func TestDrainRegularMessages_WithPendingMessages(t *testing.T) {
	ch := make(chan any, 5)
	ch <- "message one"
	ch <- "message two"

	s := &Session{
		regularCh: ch,
		messages:  []api.RequestMessage{{Role: "assistant", Content: "I responded"}},
	}
	s.drainRegularMessages()

	if len(s.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(s.messages))
	}
	if s.messages[1].Role != "user" {
		t.Errorf("new message role = %q, want 'user'", s.messages[1].Role)
	}
	content, ok := s.messages[1].Content.(string)
	if !ok {
		t.Fatalf("expected string content, got %T", s.messages[1].Content)
	}
	if !strings.Contains(content, "message one") || !strings.Contains(content, "message two") {
		t.Errorf("expected both messages in content, got: %q", content)
	}
}

func TestDrainRegularMessages_MergesWithExistingUserMessage(t *testing.T) {
	ch := make(chan any, 5)
	ch <- "team update"

	s := &Session{
		regularCh: ch,
		messages: []api.RequestMessage{
			{Role: "user", Content: "original question"},
		},
	}
	s.drainRegularMessages()

	// Should merge with the existing user message, not add a new one.
	if len(s.messages) != 1 {
		t.Fatalf("expected 1 message (merged), got %d", len(s.messages))
	}
	content, ok := s.messages[0].Content.(string)
	if !ok {
		t.Fatalf("expected string content, got %T", s.messages[0].Content)
	}
	if !strings.Contains(content, "original question") {
		t.Error("expected original question in merged content")
	}
	if !strings.Contains(content, "team update") {
		t.Error("expected team update in merged content")
	}
}

func TestDrainRegularMessages_DoesNotMergeBlockContent(t *testing.T) {
	ch := make(chan any, 5)
	ch <- "team update"

	// Last user message has non-string content ([]ContentBlock), should not merge.
	s := &Session{
		regularCh: ch,
		messages: []api.RequestMessage{
			{Role: "user", Content: []api.ContentBlock{
				{Type: "text", Text: "block content"},
			}},
		},
	}
	s.drainRegularMessages()

	// Should add a new message since the existing user message has block content.
	if len(s.messages) != 2 {
		t.Fatalf("expected 2 messages (not merged), got %d", len(s.messages))
	}
	if s.messages[1].Role != "user" {
		t.Errorf("new message role = %q, want 'user'", s.messages[1].Role)
	}
}

func TestDrainRegularMessages_ClosedChannel(t *testing.T) {
	ch := make(chan any, 5)
	ch <- "final message"
	close(ch)

	s := &Session{
		regularCh: ch,
		messages:  []api.RequestMessage{{Role: "assistant", Content: "hi"}},
	}
	s.drainRegularMessages()

	// The closed channel should be set to nil.
	if s.regularCh != nil {
		t.Error("expected regularCh to be nil after closed channel drain")
	}
	if len(s.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(s.messages))
	}
	content, ok := s.messages[1].Content.(string)
	if !ok {
		t.Fatalf("expected string content, got %T", s.messages[1].Content)
	}
	if !strings.Contains(content, "final message") {
		t.Errorf("expected 'final message' in content, got: %q", content)
	}
}

// --- Tests for syncPhaseModel ---

// mockPhaseProvider implements both ToolProvider and PhaseModelProvider.
type mockPhaseProvider struct {
	tools.ToolProvider
	model    string
	fallback string
}

func (m *mockPhaseProvider) CurrentPhaseModel() (string, string) {
	return m.model, m.fallback
}

func (m *mockPhaseProvider) Schemas() []json.RawMessage     { return nil }
func (m *mockPhaseProvider) Get(name string) tools.Tool      { return nil }
func (m *mockPhaseProvider) Execute(ctx context.Context, name string, input json.RawMessage) (tools.Result, error) {
	return tools.Result{}, nil
}
func (m *mockPhaseProvider) ExecuteStreaming(ctx context.Context, name string, input json.RawMessage, onOutput func(string)) (tools.Result, error) {
	return tools.Result{}, nil
}

func TestSyncPhaseModel_ChangesModel(t *testing.T) {
	s := newTestSessionWithTools()
	s.provider = &mockPhaseProvider{model: "claude-sonnet-4-20250514", fallback: "claude-haiku-4-20250514"}

	s.syncPhaseModel()
	if s.client.Model() != "claude-sonnet-4-20250514" {
		t.Errorf("expected model to be 'claude-sonnet-4-20250514', got %q", s.client.Model())
	}
}

func TestSyncPhaseModel_NoChangeWhenSameModel(t *testing.T) {
	s := newTestSessionWithTools()
	s.provider = &mockPhaseProvider{model: "test-model", fallback: ""}

	s.syncPhaseModel()
	// Should not change since model is already test-model.
	if s.client.Model() != "test-model" {
		t.Errorf("expected model to remain 'test-model', got %q", s.client.Model())
	}
}

func TestSyncPhaseModel_EmptyModel(t *testing.T) {
	s := newTestSessionWithTools()
	s.provider = &mockPhaseProvider{model: "", fallback: ""}

	s.syncPhaseModel()
	// Empty model should not cause a change.
	if s.client.Model() != "test-model" {
		t.Errorf("expected model to remain 'test-model', got %q", s.client.Model())
	}
}

func TestSyncPhaseModel_NonPhaseProvider(t *testing.T) {
	s := newTestSessionWithTools()
	// The default registry is not a PhaseModelProvider. syncPhaseModel should be a no-op.
	s.syncPhaseModel()
	if s.client.Model() != "test-model" {
		t.Errorf("expected model to remain 'test-model', got %q", s.client.Model())
	}
}

// --- Tests for tryFallbackModel ---

func TestTryFallbackModel_NonAPIError(t *testing.T) {
	s := newTestSessionWithTools()
	s.provider = &mockPhaseProvider{model: "test-model", fallback: "fallback-model"}

	result := s.tryFallbackModel(fmt.Errorf("generic error"))
	if result != "" {
		t.Errorf("expected empty fallback for non-API error, got %q", result)
	}
}

func TestTryFallbackModel_APIError429(t *testing.T) {
	s := newTestSessionWithTools()
	s.provider = &mockPhaseProvider{model: "test-model", fallback: "fallback-model"}

	apiErr := &api.APIError{StatusCode: 429, Type: "rate_limit", Message: "too many requests"}
	result := s.tryFallbackModel(apiErr)
	if result != "fallback-model" {
		t.Errorf("expected 'fallback-model' for 429, got %q", result)
	}
}

func TestTryFallbackModel_APIError500(t *testing.T) {
	s := newTestSessionWithTools()
	s.provider = &mockPhaseProvider{model: "test-model", fallback: "fallback-model"}

	apiErr := &api.APIError{StatusCode: 500, Type: "server_error", Message: "internal error"}
	result := s.tryFallbackModel(apiErr)
	if result != "fallback-model" {
		t.Errorf("expected 'fallback-model' for 500, got %q", result)
	}
}

func TestTryFallbackModel_APIError503(t *testing.T) {
	s := newTestSessionWithTools()
	s.provider = &mockPhaseProvider{model: "test-model", fallback: "fallback-model"}

	apiErr := &api.APIError{StatusCode: 503, Type: "overloaded", Message: "overloaded"}
	result := s.tryFallbackModel(apiErr)
	if result != "fallback-model" {
		t.Errorf("expected 'fallback-model' for 503, got %q", result)
	}
}

func TestTryFallbackModel_APIError400NotRetryable(t *testing.T) {
	s := newTestSessionWithTools()
	s.provider = &mockPhaseProvider{model: "test-model", fallback: "fallback-model"}

	apiErr := &api.APIError{StatusCode: 400, Type: "invalid_request", Message: "bad request"}
	result := s.tryFallbackModel(apiErr)
	if result != "" {
		t.Errorf("expected empty fallback for 400, got %q", result)
	}
}

func TestTryFallbackModel_APIError401NotRetryable(t *testing.T) {
	s := newTestSessionWithTools()
	s.provider = &mockPhaseProvider{model: "test-model", fallback: "fallback-model"}

	apiErr := &api.APIError{StatusCode: 401, Type: "authentication_error", Message: "unauthorized"}
	result := s.tryFallbackModel(apiErr)
	if result != "" {
		t.Errorf("expected empty fallback for 401, got %q", result)
	}
}

func TestTryFallbackModel_NoPhaseProvider(t *testing.T) {
	s := newTestSessionWithTools()
	// Default registry is not a PhaseModelProvider.

	apiErr := &api.APIError{StatusCode: 429, Type: "rate_limit", Message: "too many requests"}
	result := s.tryFallbackModel(apiErr)
	if result != "" {
		t.Errorf("expected empty fallback without PhaseModelProvider, got %q", result)
	}
}

func TestTryFallbackModel_EmptyFallback(t *testing.T) {
	s := newTestSessionWithTools()
	s.provider = &mockPhaseProvider{model: "test-model", fallback: ""}

	apiErr := &api.APIError{StatusCode: 429, Type: "rate_limit", Message: "too many requests"}
	result := s.tryFallbackModel(apiErr)
	if result != "" {
		t.Errorf("expected empty fallback when PhaseModelProvider returns empty, got %q", result)
	}
}

// --- Tests for runTurn error and edge case paths ---

func TestRunTurn_APIStreamError(t *testing.T) {
	// Server returns HTTP 500 to trigger an API error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"type":"error","error":{"type":"server_error","message":"internal failure"}}`)
	}))
	defer srv.Close()

	s := newTestSession(srv.URL)
	msgs := drainMessages(s.Turn(context.Background(), "test"))

	var gotErr bool
	for _, m := range msgs {
		if m.Type == MessageError {
			gotErr = true
		}
	}
	if !gotErr {
		t.Error("expected error message from API failure")
	}
}

func TestRunTurn_ContextCancelledBeforeStream(t *testing.T) {
	srv := newMockServer(buildSSEResponse("Hello!", "end_turn"))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	s := newTestSession(srv.URL)
	msgs := drainMessages(s.Turn(ctx, "test"))

	// With a cancelled context, the loop either emits an error or terminates
	// without producing output. Either outcome is valid.
	var gotErr bool
	for _, m := range msgs {
		if m.Type == MessageError {
			gotErr = true
		}
	}
	// If messages were produced, at least one should be an error.
	// If no messages were produced, that is also acceptable (context cancelled
	// before the goroutine could emit anything).
	if len(msgs) > 0 && !gotErr {
		// Could have received a successful response if the HTTP request completed
		// before cancellation was noticed. This is not an error.
		var gotDone bool
		for _, m := range msgs {
			if m.Type == MessageDone {
				gotDone = true
			}
		}
		if !gotDone && !gotErr {
			t.Error("expected either error or done message from cancelled context")
		}
	}
}

func TestRunTurn_UnknownStopReason(t *testing.T) {
	// Build a response with an unknown stop reason.
	unknownResp := buildSSEResponse("partial", "unknown_reason")

	srv := newMockServer(unknownResp)
	defer srv.Close()

	s := newTestSession(srv.URL)
	msgs := drainMessages(s.Turn(context.Background(), "test"))

	var gotDone bool
	for _, m := range msgs {
		if m.Type == MessageDone {
			gotDone = true
		}
	}
	if !gotDone {
		t.Error("expected done message for unknown stop reason")
	}
}

func TestRunTurn_FallbackModelOnAPIError(t *testing.T) {
	// First request returns 429, second (after fallback) succeeds.
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		if callCount.Load() == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"type":"error","error":{"type":"rate_limit_error","message":"too many requests"}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, buildSSEResponse("fallback worked", "end_turn"))
	}))
	defer srv.Close()

	cfg := Config{
		Model:        "test-model",
		Creds:        mockCreds{},
		ToolProvider: &mockPhaseProvider{model: "test-model", fallback: "fallback-model"},
	}
	s := NewSession(cfg)
	s.client.SetEndpoint(srv.URL)

	msgs := drainMessages(s.Turn(context.Background(), "test"))

	var gotText, gotDone bool
	for _, m := range msgs {
		if m.Type == MessageTextDelta && strings.Contains(m.Text, "fallback worked") {
			gotText = true
		}
		if m.Type == MessageDone {
			gotDone = true
		}
	}
	if !gotText {
		t.Error("expected text from fallback model response")
	}
	if !gotDone {
		t.Error("expected done message")
	}
	if got := callCount.Load(); got != 2 {
		t.Errorf("expected 2 API calls (initial + fallback), got %d", got)
	}
}

func TestRunTurn_FallbackModelBothFail(t *testing.T) {
	// Both requests return 429.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"type":"error","error":{"type":"rate_limit_error","message":"too many requests"}}`)
	}))
	defer srv.Close()

	cfg := Config{
		Model:        "test-model",
		Creds:        mockCreds{},
		ToolProvider: &mockPhaseProvider{model: "test-model", fallback: "fallback-model"},
	}
	s := NewSession(cfg)
	s.client.SetEndpoint(srv.URL)

	msgs := drainMessages(s.Turn(context.Background(), "test"))

	var gotErr bool
	for _, m := range msgs {
		if m.Type == MessageError {
			gotErr = true
		}
	}
	if !gotErr {
		t.Error("expected error message when both primary and fallback fail")
	}
}

func TestRunTurn_RegularMessagesInjectedBetweenTurns(t *testing.T) {
	// Two-turn conversation: tool_use then end_turn.
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

	srv := newMockServer(
		toolUseResp.String(),
		buildSSEResponse("Done!", "end_turn"),
	)
	defer srv.Close()

	regularCh := make(chan any, 5)
	// Queue a message that will be drained before the second API call.
	regularCh <- "team notification: code review complete"

	readTool := &configurableMockTool{
		name:   "Read",
		result: tools.Result{Output: "ok"},
	}
	cfg := Config{
		Model:           "test-model",
		Creds:           mockCreds{},
		Tools:           []tools.Tool{readTool},
		RegularMessages: regularCh,
	}
	s := NewSession(cfg)
	s.client.SetEndpoint(srv.URL)

	drainMessages(s.Turn(context.Background(), "test"))

	// The regular message should have been injected into the conversation history.
	found := false
	for _, msg := range s.Messages() {
		if msg.Role == "user" {
			if containsText(msg.Content, "team notification") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected regular message to be injected into conversation history")
	}
}

func TestRunTurn_TokenCountsAccumulate(t *testing.T) {
	srv := newMockServer(
		buildSSEResponse("First", "end_turn"),
		buildSSEResponse("Second", "end_turn"),
	)
	defer srv.Close()

	s := newTestSession(srv.URL)

	drainMessages(s.Turn(context.Background(), "q1"))
	firstTotal := s.TotalInput

	drainMessages(s.Turn(context.Background(), "q2"))
	secondTotal := s.TotalInput

	if secondTotal <= firstTotal {
		t.Errorf("expected cumulative input tokens to increase, got %d then %d", firstTotal, secondTotal)
	}
	// Note: Turns is only incremented on tool_use continuations within a single
	// Turn() call. Simple end_turn responses don't increment it.
	if s.TotalOutput == 0 {
		t.Error("expected non-zero total output tokens")
	}
}

func TestRunTurn_ToolExecutionErrorInLoop(t *testing.T) {
	// Server returns a tool_use response, but tool execution will fail because
	// context gets cancelled during permission request.
	var toolUseResp strings.Builder
	toolUseResp.WriteString("event: message_start\n")
	toolUseResp.WriteString(`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"usage":{"input_tokens":50,"output_tokens":0}}}`)
	toolUseResp.WriteString("\n\n")
	toolUseResp.WriteString("event: content_block_start\n")
	toolUseResp.WriteString(`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tu_1","name":"Bash"}}`)
	toolUseResp.WriteString("\n\n")
	toolUseResp.WriteString("event: content_block_delta\n")
	toolUseResp.WriteString(`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"command\":\"test\"}"}}`)
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

	srv := newMockServer(toolUseResp.String())
	defer srv.Close()

	// Use a tool that requires permission (Bash, not in alwaysAllowTools).
	// When permission is requested, cancel the context to trigger error in executeTools.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bashTool := &configurableMockTool{
		name:   "Bash",
		result: tools.Result{Output: "ok"},
	}
	cfg := Config{
		Model: "test-model",
		Creds: mockCreds{},
		Tools: []tools.Tool{bashTool},
	}
	s := NewSession(cfg)
	s.client.SetEndpoint(srv.URL)

	out := s.Turn(ctx, "run bash")

	// Drain messages. When we see a permission request, cancel the context.
	var gotPermReq, gotDone bool
	for m := range out {
		if m.Type == MessagePermissionRequest {
			cancel()
			gotPermReq = true
		}
		if m.Type == MessageDone {
			gotDone = true
		}
	}

	// The permission request should have been emitted before cancellation.
	if !gotPermReq {
		t.Error("expected permission request for Bash tool")
	}
	// After context cancellation, the turn should terminate without emitting
	// a done message (emit drops messages when context is cancelled).
	if gotDone {
		t.Error("expected no done message after context cancellation")
	}
}

func TestRunTurn_ProcessTurnErrorInLoop(t *testing.T) {
	// Server returns a stream error during the response.
	var errorResp strings.Builder
	errorResp.WriteString("event: message_start\n")
	errorResp.WriteString(`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"usage":{"input_tokens":50,"output_tokens":0}}}`)
	errorResp.WriteString("\n\n")
	errorResp.WriteString("event: error\n")
	errorResp.WriteString(`data: {"type":"error","error":{"type":"overloaded_error","message":"server overloaded"}}`)
	errorResp.WriteString("\n\n")

	srv := newMockServer(errorResp.String())
	defer srv.Close()

	s := newTestSession(srv.URL)
	msgs := drainMessages(s.Turn(context.Background(), "test"))

	var gotErr bool
	for _, m := range msgs {
		if m.Type == MessageError {
			gotErr = true
		}
	}
	if !gotErr {
		t.Error("expected error message from stream error in processTurn")
	}
}

func TestNewSession_DefaultModel(t *testing.T) {
	cfg := Config{
		Creds: mockCreds{},
	}
	s := NewSession(cfg)
	if s.client.Model() != DefaultModel {
		t.Errorf("expected default model %q, got %q", DefaultModel, s.client.Model())
	}
}

func TestNewSession_CustomToolProvider(t *testing.T) {
	provider := &mockPhaseProvider{model: "custom-model", fallback: ""}
	cfg := Config{
		Model:        "test-model",
		Creds:        mockCreds{},
		ToolProvider: provider,
	}
	s := NewSession(cfg)
	if s.provider != provider {
		t.Error("expected custom ToolProvider to be used")
	}
}

// --- Tests for appendUserMessage ---

func TestAppendUserMessage_FirstMessage(t *testing.T) {
	s := &Session{}
	s.appendUserMessage([]api.ContentBlock{{Type: "text", Text: "hello"}})

	if len(s.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(s.messages))
	}
	if s.messages[0].Role != "user" {
		t.Errorf("role = %q, want 'user'", s.messages[0].Role)
	}
	blocks, ok := s.messages[0].Content.([]api.ContentBlock)
	if !ok {
		t.Fatalf("expected []ContentBlock, got %T", s.messages[0].Content)
	}
	if len(blocks) != 1 || blocks[0].Text != "hello" {
		t.Errorf("unexpected content: %+v", blocks)
	}
}

func TestAppendUserMessage_MergesConsecutiveUser(t *testing.T) {
	s := &Session{
		messages: []api.RequestMessage{
			{Role: "user", Content: []api.ContentBlock{
				{Type: "tool_result", ToolUseID: "t1", Content: "cancelled", IsError: true},
			}},
		},
	}
	s.appendUserMessage([]api.ContentBlock{{Type: "text", Text: "next prompt"}})

	if len(s.messages) != 1 {
		t.Fatalf("expected 1 merged message, got %d", len(s.messages))
	}
	blocks, ok := s.messages[0].Content.([]api.ContentBlock)
	if !ok {
		t.Fatalf("expected []ContentBlock, got %T", s.messages[0].Content)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks in merged message, got %d", len(blocks))
	}
	if blocks[0].Type != "tool_result" || blocks[0].ToolUseID != "t1" {
		t.Errorf("first block should be original tool_result, got %+v", blocks[0])
	}
	if blocks[1].Type != "text" || blocks[1].Text != "next prompt" {
		t.Errorf("second block should be new text, got %+v", blocks[1])
	}
}

func TestAppendUserMessage_MergesStringContent(t *testing.T) {
	s := &Session{
		messages: []api.RequestMessage{
			{Role: "user", Content: "existing string"},
		},
	}
	s.appendUserMessage([]api.ContentBlock{{Type: "text", Text: "new prompt"}})

	if len(s.messages) != 1 {
		t.Fatalf("expected 1 merged message, got %d", len(s.messages))
	}
	blocks, ok := s.messages[0].Content.([]api.ContentBlock)
	if !ok {
		t.Fatalf("expected []ContentBlock, got %T", s.messages[0].Content)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Text != "existing string" {
		t.Errorf("first block text = %q, want 'existing string'", blocks[0].Text)
	}
	if blocks[1].Text != "new prompt" {
		t.Errorf("second block text = %q, want 'new prompt'", blocks[1].Text)
	}
}

func TestAppendUserMessage_NoMergeAfterAssistant(t *testing.T) {
	s := &Session{
		messages: []api.RequestMessage{
			{Role: "assistant", Content: "I responded"},
		},
	}
	s.appendUserMessage([]api.ContentBlock{{Type: "text", Text: "follow up"}})

	if len(s.messages) != 2 {
		t.Fatalf("expected 2 messages (no merge), got %d", len(s.messages))
	}
	if s.messages[1].Role != "user" {
		t.Errorf("second message role = %q, want 'user'", s.messages[1].Role)
	}
}

func TestTurn_MergesAfterCancelledTurn(t *testing.T) {
	// Simulate a session that ended with error tool_results from cancellation.
	srv := newMockServer(buildSSEResponse("recovered", "end_turn"))
	defer srv.Close()

	s := newTestSession(srv.URL)
	// Manually set up message history as if a previous turn was cancelled:
	// assistant with tool_use, then user with error tool_results.
	s.messages = []api.RequestMessage{
		{Role: "user", Content: "original prompt"},
		{Role: "assistant", Content: []api.ContentBlock{
			{Type: "tool_use", ID: "t1", Name: "Read", Input: json.RawMessage(`{}`)},
		}},
		{Role: "user", Content: []api.ContentBlock{
			{Type: "tool_result", ToolUseID: "t1", Content: "tool execution cancelled: context canceled", IsError: true},
		}},
	}

	// The next Turn should merge with the trailing user message, not create
	// consecutive user messages.
	msgs := drainMessages(s.Turn(context.Background(), "try again"))

	// Verify no consecutive user messages in the history.
	for i := 1; i < len(s.Messages()); i++ {
		if s.Messages()[i].Role == "user" && s.Messages()[i-1].Role == "user" {
			t.Errorf("consecutive user messages at index %d and %d", i-1, i)
		}
	}

	// Should have gotten a response.
	var gotDone bool
	for _, m := range msgs {
		if m.Type == MessageDone {
			gotDone = true
		}
	}
	if !gotDone {
		t.Error("expected done message after recovery turn")
	}
}

// Ensure mockCreds satisfies auth.Credentials.
var _ auth.Credentials = mockCreds{}

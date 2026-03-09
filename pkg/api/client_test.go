package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/justin/glamdring/pkg/auth"
)

func TestStreamNoGoroutineLeak(t *testing.T) {
	server := newMockSSEServer(buildSSEResponse("hello", "end_turn"))
	defer server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "test-model")
	client.SetEndpoint(server.URL)

	// Stabilize goroutine count.
	runtime.GC()
	time.Sleep(10 * time.Millisecond)
	before := runtime.NumGoroutine()

	ctx := context.Background()
	events, err := client.Stream(ctx, &MessageRequest{
		MaxTokens: 1024,
		Messages:  []RequestMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	// Drain all events.
	for range events {
	}

	// Close the server so its goroutines clean up before measuring.
	server.Close()

	// Wait for goroutines to clean up.
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	time.Sleep(10 * time.Millisecond)
	after := runtime.NumGoroutine()

	// Allow +1 tolerance for runtime background goroutines.
	if after > before+1 {
		t.Errorf("goroutine leak: before=%d, after=%d", before, after)
	}
}

func TestStreamContextCancellation(t *testing.T) {
	// Server that sends events slowly — we cancel mid-stream.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		// Send message_start.
		fmt.Fprint(w, "event: message_start\n")
		fmt.Fprint(w, `data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":0}}}`)
		fmt.Fprint(w, "\n\n")
		flusher.Flush()

		// Block until client disconnects.
		<-r.Context().Done()
	}))
	defer server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "test-model")
	client.SetEndpoint(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	events, err := client.Stream(ctx, &MessageRequest{
		MaxTokens: 1024,
		Messages:  []RequestMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	// Read one event, then cancel.
	<-events
	cancel()

	// Channel should close promptly.
	timeout := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-events:
			if !ok {
				return // success — channel closed
			}
		case <-timeout:
			t.Fatal("events channel not closed after context cancellation")
		}
	}
}

// newMockSSEServer creates a test server that returns canned SSE responses.
// Each call to the server returns the next response in the sequence.
// If the sequence is exhausted, it returns the last response.
func newMockSSEServer(responses ...string) *httptest.Server {
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

// buildSSEResponse constructs a complete SSE stream for a simple text response
// with the given text and stop reason.
func buildSSEResponse(text, stopReason string) string {
	var b strings.Builder

	// message_start
	b.WriteString("event: message_start\n")
	b.WriteString(`data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":100,"output_tokens":0}}}`)
	b.WriteString("\n\n")

	// content_block_start
	b.WriteString("event: content_block_start\n")
	b.WriteString(`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
	b.WriteString("\n\n")

	// content_block_delta
	b.WriteString("event: content_block_delta\n")
	b.WriteString(fmt.Sprintf(`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"%s"}}`, text))
	b.WriteString("\n\n")

	// content_block_stop
	b.WriteString("event: content_block_stop\n")
	b.WriteString(`data: {"type":"content_block_stop","index":0}`)
	b.WriteString("\n\n")

	// message_delta
	b.WriteString("event: message_delta\n")
	b.WriteString(fmt.Sprintf(`data: {"type":"message_delta","delta":{"stop_reason":"%s"},"usage":{"output_tokens":50}}`, stopReason))
	b.WriteString("\n\n")

	// message_stop
	b.WriteString("event: message_stop\n")
	b.WriteString(`data: {"type":"message_stop"}`)
	b.WriteString("\n\n")

	return b.String()
}

func TestModelAndSetModel(t *testing.T) {
	client := NewClient(&auth.APIKeyCredentials{Key: "k"}, "claude-3-haiku")
	if got := client.Model(); got != "claude-3-haiku" {
		t.Errorf("Model() = %q, want %q", got, "claude-3-haiku")
	}
	client.SetModel("claude-opus-4")
	if got := client.Model(); got != "claude-opus-4" {
		t.Errorf("after SetModel, Model() = %q, want %q", got, "claude-opus-4")
	}
}

func TestSupportsThinking(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"claude-opus-4-20250514", true},
		{"claude-sonnet-4-20250514", true},
		{"claude-opus-4-6", true},
		{"claude-sonnet-4-6", true},
		{"claude-3-7-sonnet-20250219", true},
		{"claude-3-haiku-20240307", false},
		{"claude-3-opus-20240229", false},
		{"test-model", false},
	}
	for _, tt := range tests {
		client := NewClient(&auth.APIKeyCredentials{Key: "k"}, tt.model)
		if got := client.supportsThinking(); got != tt.want {
			t.Errorf("supportsThinking(%q) = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestSupportsAdaptiveThinking(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"claude-opus-4-6", true},
		{"claude-sonnet-4-6", true},
		{"claude-opus-4-20250514", false},
		{"claude-sonnet-4-20250514", false},
		{"claude-3-7-sonnet-20250219", false},
		{"test-model", false},
	}
	for _, tt := range tests {
		client := NewClient(&auth.APIKeyCredentials{Key: "k"}, tt.model)
		if got := client.supportsAdaptiveThinking(); got != tt.want {
			t.Errorf("supportsAdaptiveThinking(%q) = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestParseAPIError(t *testing.T) {
	t.Run("valid error JSON", func(t *testing.T) {
		body := `{"type":"error","error":{"type":"rate_limit_error","message":"too many requests"}}`
		apiErr := parseAPIError(429, []byte(body))
		if apiErr.StatusCode != 429 {
			t.Errorf("StatusCode = %d, want 429", apiErr.StatusCode)
		}
		if apiErr.Type != "rate_limit_error" {
			t.Errorf("Type = %q, want %q", apiErr.Type, "rate_limit_error")
		}
		if apiErr.Message != "too many requests" {
			t.Errorf("Message = %q, want %q", apiErr.Message, "too many requests")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		apiErr := parseAPIError(500, []byte("not json"))
		if apiErr.StatusCode != 500 {
			t.Errorf("StatusCode = %d, want 500", apiErr.StatusCode)
		}
		if apiErr.Type != "unknown" {
			t.Errorf("Type = %q, want %q", apiErr.Type, "unknown")
		}
		if !strings.Contains(apiErr.Message, "not json") {
			t.Errorf("Message = %q, should contain the raw body", apiErr.Message)
		}
	})

	t.Run("valid JSON but no error message", func(t *testing.T) {
		body := `{"type":"error","error":{"type":"unknown_error"}}`
		apiErr := parseAPIError(400, []byte(body))
		if apiErr.Type != "unknown" {
			t.Errorf("Type = %q, want %q (empty message falls through)", apiErr.Type, "unknown")
		}
	})

	t.Run("empty body", func(t *testing.T) {
		apiErr := parseAPIError(502, []byte(""))
		if apiErr.StatusCode != 502 {
			t.Errorf("StatusCode = %d, want 502", apiErr.StatusCode)
		}
		if apiErr.Type != "unknown" {
			t.Errorf("Type = %q, want %q", apiErr.Type, "unknown")
		}
	})
}

func TestDoWithRetrySuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers are set.
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want %q", r.Header.Get("Content-Type"), "application/json")
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("x-api-key = %q, want %q", r.Header.Get("x-api-key"), "test-key")
		}
		if r.Header.Get("anthropic-version") != anthropicVersion {
			t.Errorf("anthropic-version = %q, want %q", r.Header.Get("anthropic-version"), anthropicVersion)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "test-model")
	client.SetEndpoint(server.URL)

	resp, err := client.doWithRetry(context.Background(), []byte(`{"test":true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"ok":true}` {
		t.Errorf("body = %q, want %q", string(body), `{"ok":true}`)
	}
}

func TestDoWithRetryRetriesOnRetryableStatus(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"type":"error","error":{"type":"server_error","message":"internal error"}}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "test-model")
	client.SetEndpoint(server.URL)

	resp, err := client.doWithRetry(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if got := attempts.Load(); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

func TestDoWithRetryExhaustsRetries(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"type":"error","error":{"type":"server_error","message":"always failing"}}`)
	}))
	defer server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "test-model")
	client.SetEndpoint(server.URL)

	_, err := client.doWithRetry(context.Background(), []byte(`{}`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "max retries exceeded") {
		t.Errorf("error = %q, want to contain 'max retries exceeded'", err.Error())
	}
	// Should try maxRetries+1 times.
	if got := attempts.Load(); got != int32(maxRetries+1) {
		t.Errorf("attempts = %d, want %d", got, maxRetries+1)
	}
}

func TestDoWithRetryNonRetryableError(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"type":"error","error":{"type":"invalid_request","message":"bad request"}}`)
	}))
	defer server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "test-model")
	client.SetEndpoint(server.URL)

	_, err := client.doWithRetry(context.Background(), []byte(`{}`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error is %T, want *APIError", err)
	}
	if apiErr.StatusCode != 400 {
		t.Errorf("StatusCode = %d, want 400", apiErr.StatusCode)
	}
	if apiErr.Type != "invalid_request" {
		t.Errorf("Type = %q, want %q", apiErr.Type, "invalid_request")
	}
	// Should not retry for 400.
	if got := attempts.Load(); got != 1 {
		t.Errorf("attempts = %d, want 1 (no retry)", got)
	}
}

func TestDoWithRetryContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"type":"error","error":{"type":"server_error","message":"fail"}}`)
	}))
	defer server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "test-model")
	client.SetEndpoint(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so the sleep in retry is interrupted.
	cancel()

	_, err := client.doWithRetry(ctx, []byte(`{}`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// The error could be context.Canceled or a wrapped context error.
	if !strings.Contains(err.Error(), "canceled") && !strings.Contains(err.Error(), "context") {
		t.Errorf("error = %q, want context cancellation error", err.Error())
	}
}

func TestDoWithRetryRateLimitWithRetryAfterHeader(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"type":"error","error":{"type":"rate_limit_error","message":"rate limited"}}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "test-model")
	client.SetEndpoint(server.URL)

	resp, err := client.doWithRetry(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if got := attempts.Load(); got != 2 {
		t.Errorf("attempts = %d, want 2", got)
	}
}

func TestDoWithRetryOverloadedStatus(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.WriteHeader(529) // Overloaded
			fmt.Fprint(w, `{"type":"error","error":{"type":"overloaded_error","message":"overloaded"}}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "test-model")
	client.SetEndpoint(server.URL)

	resp, err := client.doWithRetry(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if got := attempts.Load(); got != 2 {
		t.Errorf("attempts = %d, want 2", got)
	}
}

func TestDoWithRetryNetworkError(t *testing.T) {
	// Create a server and close it immediately to force network errors.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	serverURL := server.URL
	server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "test-model")
	client.SetEndpoint(serverURL)

	// Use a generous timeout: maxRetries=3 means 4 attempts, with backoff up to 8s total.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := client.doWithRetry(ctx, []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for closed server")
	}
	if !strings.Contains(err.Error(), "max retries exceeded") {
		t.Errorf("error = %q, want to contain 'max retries exceeded'", err.Error())
	}
}

func TestStreamSetsModelAndStream(t *testing.T) {
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, buildSSEResponse("hi", "end_turn"))
	}))
	defer server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "my-model")
	client.SetEndpoint(server.URL)

	events, err := client.Stream(context.Background(), &MessageRequest{
		MaxTokens: 512,
		Messages:  []RequestMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	for range events {
	}

	if receivedBody["model"] != "my-model" {
		t.Errorf("model = %v, want %q", receivedBody["model"], "my-model")
	}
	if receivedBody["stream"] != true {
		t.Errorf("stream = %v, want true", receivedBody["stream"])
	}
}

func TestStreamEnablesThinkingForSupportedModel(t *testing.T) {
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, buildSSEResponse("hi", "end_turn"))
	}))
	defer server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "claude-opus-4-20250514")
	client.SetEndpoint(server.URL)

	events, err := client.Stream(context.Background(), &MessageRequest{
		MaxTokens: 512,
		Messages:  []RequestMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	for range events {
	}

	thinking, ok := receivedBody["thinking"].(map[string]any)
	if !ok {
		t.Fatalf("thinking not set in request body")
	}
	if thinking["type"] != "enabled" {
		t.Errorf("thinking.type = %v, want %q", thinking["type"], "enabled")
	}
}

func TestStreamAdaptiveThinkingForOpus46(t *testing.T) {
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, buildSSEResponse("hi", "end_turn"))
	}))
	defer server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "claude-opus-4-6")
	client.SetEndpoint(server.URL)

	events, err := client.Stream(context.Background(), &MessageRequest{
		MaxTokens: 16384,
		Messages:  []RequestMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	for range events {
	}

	thinking, ok := receivedBody["thinking"].(map[string]any)
	if !ok {
		t.Fatalf("thinking not set in request body")
	}
	if thinking["type"] != "adaptive" {
		t.Errorf("thinking.type = %v, want \"adaptive\"", thinking["type"])
	}
	if _, hasBudget := thinking["budget_tokens"]; hasBudget {
		t.Error("adaptive thinking should not include budget_tokens")
	}
}

func TestStreamUsesConfiguredThinkingBudget(t *testing.T) {
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, buildSSEResponse("hi", "end_turn"))
	}))
	defer server.Close()

	// Use a budget-based model (not 4-6).
	budget := 5000
	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "claude-opus-4-20250514")
	client.SetEndpoint(server.URL)
	client.SetThinkingBudget(&budget)

	events, err := client.Stream(context.Background(), &MessageRequest{
		MaxTokens: 16384,
		Messages:  []RequestMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	for range events {
	}

	thinking, ok := receivedBody["thinking"].(map[string]any)
	if !ok {
		t.Fatalf("thinking not set in request body")
	}
	if thinking["budget_tokens"] != float64(5000) {
		t.Errorf("thinking.budget_tokens = %v, want 5000", thinking["budget_tokens"])
	}
}

func TestStreamZeroBudgetDisablesThinking(t *testing.T) {
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, buildSSEResponse("hi", "end_turn"))
	}))
	defer server.Close()

	zero := 0
	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "claude-opus-4-20250514")
	client.SetEndpoint(server.URL)
	client.SetThinkingBudget(&zero)

	events, err := client.Stream(context.Background(), &MessageRequest{
		MaxTokens: 16384,
		Messages:  []RequestMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	for range events {
	}

	if _, ok := receivedBody["thinking"]; ok {
		t.Error("thinking should not be set when budget is 0")
	}
}

func TestStreamDoesNotOverrideExistingThinking(t *testing.T) {
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, buildSSEResponse("hi", "end_turn"))
	}))
	defer server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "claude-opus-4-20250514")
	client.SetEndpoint(server.URL)

	events, err := client.Stream(context.Background(), &MessageRequest{
		MaxTokens: 512,
		Messages:  []RequestMessage{{Role: "user", Content: "hello"}},
		Thinking:  &ThinkingConfig{Type: "enabled", BudgetTokens: 50000},
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	for range events {
	}

	thinking, ok := receivedBody["thinking"].(map[string]any)
	if !ok {
		t.Fatalf("thinking not set in request body")
	}
	if thinking["budget_tokens"] != float64(50000) {
		t.Errorf("thinking.budget_tokens = %v, want 50000 (should not be overwritten)", thinking["budget_tokens"])
	}
}

// mockOAuthCredentials implements auth.Credentials with controllable OAuth behavior.
type mockOAuthCredentials struct {
	token        string
	refreshErr   error
	refreshCount int
}

func (m *mockOAuthCredentials) SetAuthHeaders(req *http.Request) error {
	req.Header.Set("Authorization", "Bearer "+m.token)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	return nil
}

func (m *mockOAuthCredentials) IsOAuth() bool { return true }

func (m *mockOAuthCredentials) Refresh() error {
	m.refreshCount++
	if m.refreshErr != nil {
		return m.refreshErr
	}
	m.token = "refreshed-token"
	return nil
}

func TestDoWithRetryOAuth401RefreshSuccess(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			// First request: 401 unauthorized.
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid token"}}`)
			return
		}
		// Second request (after refresh): check for refreshed token.
		if r.Header.Get("Authorization") != "Bearer refreshed-token" {
			t.Errorf("expected refreshed token, got %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	creds := &mockOAuthCredentials{token: "expired-token"}
	client := NewClient(creds, "test-model")
	client.SetEndpoint(server.URL)

	resp, err := client.doWithRetry(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if creds.refreshCount != 1 {
		t.Errorf("refreshCount = %d, want 1", creds.refreshCount)
	}
	if got := attempts.Load(); got != 2 {
		t.Errorf("attempts = %d, want 2", got)
	}
}

func TestDoWithRetryOAuth401RefreshFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid token"}}`)
	}))
	defer server.Close()

	creds := &mockOAuthCredentials{
		token:      "expired-token",
		refreshErr: fmt.Errorf("refresh failed"),
	}
	client := NewClient(creds, "test-model")
	client.SetEndpoint(server.URL)

	_, err := client.doWithRetry(context.Background(), []byte(`{}`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error is %T, want *APIError", err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", apiErr.StatusCode)
	}
}

func TestDoWithRetryOAuth401RetryStillFails(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"type":"error","error":{"type":"authentication_error","message":"still invalid"}}`)
	}))
	defer server.Close()

	creds := &mockOAuthCredentials{token: "expired-token"}
	client := NewClient(creds, "test-model")
	client.SetEndpoint(server.URL)

	_, err := client.doWithRetry(context.Background(), []byte(`{}`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error is %T, want *APIError", err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", apiErr.StatusCode)
	}
	// Should be exactly 2: initial + 1 retry after refresh.
	if got := attempts.Load(); got != 2 {
		t.Errorf("attempts = %d, want 2", got)
	}
}

func TestDoWithRetryNonOAuth401(t *testing.T) {
	// With API key creds (not OAuth), 401 should not retry.
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"type":"error","error":{"type":"authentication_error","message":"bad key"}}`)
	}))
	defer server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "bad-key"}, "test-model")
	client.SetEndpoint(server.URL)

	_, err := client.doWithRetry(context.Background(), []byte(`{}`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error is %T, want *APIError", err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", apiErr.StatusCode)
	}
	// Should be exactly 1: no retry for non-OAuth 401.
	if got := attempts.Load(); got != 1 {
		t.Errorf("attempts = %d, want 1", got)
	}
}

func TestDoWithRetryOAuth401RetryNetworkError(t *testing.T) {
	// Server returns 401 on first request, then we simulate a network error
	// on the retry after refresh.
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid token"}}`)
	}))

	creds := &mockOAuthCredentials{token: "expired-token"}
	client := NewClient(creds, "test-model")
	client.SetEndpoint(server.URL)

	// After first call hits the server, close it so the retry gets a network error.
	origTransport := http.DefaultTransport
	client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		resp, err := origTransport.RoundTrip(req)
		if err == nil && callCount.Load() == 1 {
			// After first 401, close server so retry gets a network error.
			server.Close()
		}
		return resp, err
	})

	_, err := client.doWithRetry(context.Background(), []byte(`{}`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "http request after token refresh") {
		t.Errorf("error = %q, want to contain 'http request after token refresh'", err.Error())
	}
}

// roundTripFunc wraps a function as an http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDoWithRetryRateLimitWithPositiveRetryAfter(t *testing.T) {
	// 429 with a positive Retry-After header value.
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"type":"error","error":{"type":"rate_limit_error","message":"rate limited"}}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "test-model")
	client.SetEndpoint(server.URL)

	start := time.Now()
	resp, err := client.doWithRetry(context.Background(), []byte(`{}`))
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if got := attempts.Load(); got != 2 {
		t.Errorf("attempts = %d, want 2", got)
	}
	// Should have waited about 1 second from the Retry-After header.
	if elapsed < 900*time.Millisecond {
		t.Errorf("elapsed = %v, expected at least ~1s from Retry-After", elapsed)
	}
}

func TestDoWithRetryContextCancelDuringRetryableBackoff(t *testing.T) {
	// Return a retryable error, then cancel context during the backoff sleep.
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"type":"error","error":{"type":"server_error","message":"fail"}}`)
	}))
	defer server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "test-model")
	client.SetEndpoint(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := client.doWithRetry(ctx, []byte(`{}`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Should have been cancelled during backoff, not exhausted retries.
	if got := attempts.Load(); got > int32(maxRetries) {
		t.Errorf("attempts = %d, should not exhaust all retries before context cancel", got)
	}
}

func TestStreamAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"type":"error","error":{"type":"invalid_request","message":"bad"}}`)
	}))
	defer server.Close()

	client := NewClient(&auth.APIKeyCredentials{Key: "test-key"}, "test-model")
	client.SetEndpoint(server.URL)

	_, err := client.Stream(context.Background(), &MessageRequest{
		MaxTokens: 512,
		Messages:  []RequestMessage{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error is %T, want *APIError", err)
	}
	if apiErr.StatusCode != 400 {
		t.Errorf("StatusCode = %d, want 400", apiErr.StatusCode)
	}
}

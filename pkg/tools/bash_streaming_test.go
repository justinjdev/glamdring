package tools

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestBashTool_ExecuteStreaming_Basic(t *testing.T) {
	bash := BashTool{CWD: t.TempDir()}
	input, _ := json.Marshal(bashInput{Command: "echo hello; echo world"})

	var mu sync.Mutex
	var deltas []string
	onOutput := func(text string) {
		mu.Lock()
		deltas = append(deltas, text)
		mu.Unlock()
	}

	result, err := bash.ExecuteStreaming(context.Background(), input, onOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected no error, got: %s", result.Output)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(deltas) < 2 {
		t.Errorf("expected at least 2 deltas, got %d: %v", len(deltas), deltas)
	}
	combined := strings.Join(deltas, "")
	if !strings.Contains(combined, "hello") || !strings.Contains(combined, "world") {
		t.Errorf("expected hello and world in deltas, got: %s", combined)
	}
}

func TestBashTool_ExecuteStreaming_Stderr(t *testing.T) {
	bash := BashTool{CWD: t.TempDir()}
	input, _ := json.Marshal(bashInput{Command: "echo out; echo err >&2"})

	var mu sync.Mutex
	var deltas []string
	onOutput := func(text string) {
		mu.Lock()
		deltas = append(deltas, text)
		mu.Unlock()
	}

	result, err := bash.ExecuteStreaming(context.Background(), input, onOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	combined := strings.Join(deltas, "")
	if !strings.Contains(combined, "out") {
		t.Errorf("expected stdout in deltas, got: %s", combined)
	}
	if !strings.Contains(combined, "err") {
		t.Errorf("expected stderr in deltas, got: %s", combined)
	}
	if !strings.Contains(result.Output, "out") {
		t.Errorf("expected stdout in result, got: %s", result.Output)
	}
}

func TestBashTool_ExecuteStreaming_Background_FallsBack(t *testing.T) {
	bash := BashTool{CWD: t.TempDir()}
	input, _ := json.Marshal(bashInput{Command: "echo bg", RunInBackground: true})

	called := false
	onOutput := func(text string) {
		called = true
	}

	result, err := bash.ExecuteStreaming(context.Background(), input, onOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("expected onOutput not to be called for background commands")
	}
	if !strings.Contains(result.Output, "background process started") {
		t.Errorf("expected background result, got: %s", result.Output)
	}
}

func TestBashTool_ExecuteStreaming_ExitCode(t *testing.T) {
	bash := BashTool{CWD: t.TempDir()}
	input, _ := json.Marshal(bashInput{Command: "echo fail; exit 1"})

	var mu sync.Mutex
	var deltas []string
	onOutput := func(text string) {
		mu.Lock()
		deltas = append(deltas, text)
		mu.Unlock()
	}

	result, err := bash.ExecuteStreaming(context.Background(), input, onOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError for non-zero exit code")
	}
	if !strings.Contains(result.Output, "exit code: 1") {
		t.Errorf("expected exit code in output, got: %s", result.Output)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(deltas) == 0 {
		t.Error("expected at least one delta before exit")
	}
}

func TestBashTool_ExecuteStreaming_Timeout(t *testing.T) {
	bash := BashTool{CWD: t.TempDir()}
	// Use a very short timeout so the test runs quickly.
	input, _ := json.Marshal(bashInput{Command: "sleep 30", Timeout: 100})

	var mu sync.Mutex
	var deltas []string
	onOutput := func(text string) {
		mu.Lock()
		deltas = append(deltas, text)
		mu.Unlock()
	}

	result, err := bash.ExecuteStreaming(context.Background(), input, onOutput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError for timed-out command")
	}
	if result.Output != "command timed out" {
		t.Errorf("expected 'command timed out', got: %s", result.Output)
	}
}

func TestRegistry_ExecuteStreaming_FallsBackForNonStreaming(t *testing.T) {
	reg := NewRegistry()

	// Register a plain (non-streaming) tool.
	reg.Register(&mockTool{name: "test", output: "result"})

	result, err := reg.ExecuteStreaming(context.Background(), "test", json.RawMessage("{}"), func(string) {
		t.Error("onOutput should not be called for non-streaming tools")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "result" {
		t.Errorf("expected 'result', got: %s", result.Output)
	}
}

// mockTool is a minimal Tool implementation for testing.
type mockTool struct {
	name   string
	output string
}

func (m *mockTool) Name() string                   { return m.name }
func (m *mockTool) Description() string            { return "mock" }
func (m *mockTool) Schema() json.RawMessage        { return json.RawMessage(`{"type":"object"}`) }
func (m *mockTool) Execute(_ context.Context, _ json.RawMessage) (Result, error) {
	return Result{Output: m.output}, nil
}

package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBashTimeoutDetection(t *testing.T) {
	tool := BashTool{CWD: "/tmp"}
	input, _ := json.Marshal(bashInput{
		Command: "sleep 10",
		Timeout: 500, // 500ms
	})

	start := time.Now()
	result, err := tool.Execute(context.Background(), input)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError for timed-out command")
	}
	if result.Output != "command timed out" {
		t.Errorf("expected 'command timed out', got %q", result.Output)
	}
	if elapsed > 5*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestBashTimeoutCap(t *testing.T) {
	tool := BashTool{CWD: "/tmp"}
	// Provide a timeout > 600000ms — should be capped.
	input, _ := json.Marshal(bashInput{
		Command: "echo hello",
		Timeout: 999999,
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Output)
	}
	if !strings.Contains(result.Output, "hello") {
		t.Errorf("expected 'hello' in output, got %q", result.Output)
	}
}

func TestBashOutputTruncation(t *testing.T) {
	tool := BashTool{CWD: "/tmp"}
	// Generate > 1MB of output.
	input, _ := json.Marshal(bashInput{
		Command: "yes 'this is a long line of repeated output for testing truncation purposes' | head -20000",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Output) > maxOutputSize+1000 { // allow some overhead for truncation message
		t.Errorf("output not truncated: %d bytes", len(result.Output))
	}
	if strings.Contains(result.Output, "truncated") {
		// Verify it mentions truncation.
		if !strings.Contains(result.Output, "showing last") {
			t.Error("truncation message missing 'showing last' text")
		}
	}
}

func TestBashRunInBackground(t *testing.T) {
	tool := BashTool{CWD: "/tmp"}
	input, _ := json.Marshal(bashInput{
		Command:        "sleep 0.1 && echo done",
		RunInBackground: true,
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error result: %s", result.Output)
	}
	if !strings.Contains(result.Output, "background process started with PID") {
		t.Errorf("expected PID in output, got %q", result.Output)
	}
}

func TestBashEmptyCommand(t *testing.T) {
	tool := BashTool{CWD: "/tmp"}
	input, _ := json.Marshal(bashInput{Command: ""})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for empty command")
	}
}

func TestBashExitCode(t *testing.T) {
	tool := BashTool{CWD: "/tmp"}
	input, _ := json.Marshal(bashInput{Command: "exit 42"})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError for non-zero exit")
	}
	if !strings.Contains(result.Output, "exit code: 42") {
		t.Errorf("expected exit code 42 in output, got %q", result.Output)
	}
}

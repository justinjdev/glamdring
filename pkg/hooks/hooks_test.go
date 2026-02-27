package hooks

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewHookRunner_BadRegexSkipped(t *testing.T) {
	hooks := []Hook{
		{Event: PreToolUse, Matcher: "[invalid", Command: "echo bad"},
		{Event: PreToolUse, Matcher: "Edit", Command: "echo good"},
	}
	r := NewHookRunner(hooks)
	if len(r.hooks) != 1 {
		t.Fatalf("expected 1 compiled hook, got %d", len(r.hooks))
	}
	if r.hooks[0].hook.Matcher != "Edit" {
		t.Errorf("expected surviving hook matcher to be 'Edit', got %q", r.hooks[0].hook.Matcher)
	}
}

func TestNewHookRunner_EmptyMatcherMatchesAll(t *testing.T) {
	hooks := []Hook{
		{Event: PostToolUse, Matcher: "", Command: "true"},
	}
	r := NewHookRunner(hooks)
	if len(r.hooks) != 1 {
		t.Fatalf("expected 1 compiled hook, got %d", len(r.hooks))
	}
	if r.hooks[0].matcher != nil {
		t.Error("expected nil matcher for empty pattern")
	}
}

func TestRun_PreToolUse_MatchAndBlock(t *testing.T) {
	hooks := []Hook{
		{Event: PreToolUse, Matcher: "Bash", Command: "exit 1"},
	}
	r := NewHookRunner(hooks)

	err := r.Run(context.Background(), PreToolUse, "Bash", nil)
	if err == nil {
		t.Fatal("expected error from failing PreToolUse hook")
	}
	if !strings.Contains(err.Error(), "PreToolUse hook") {
		t.Errorf("error should mention PreToolUse hook: %v", err)
	}
}

func TestRun_PreToolUse_NoMatchAllowed(t *testing.T) {
	hooks := []Hook{
		{Event: PreToolUse, Matcher: "Bash", Command: "exit 1"},
	}
	r := NewHookRunner(hooks)

	// Tool name "Read" doesn't match "Bash", so no error.
	err := r.Run(context.Background(), PreToolUse, "Read", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_PostToolUse_FailureDoesNotBlock(t *testing.T) {
	hooks := []Hook{
		{Event: PostToolUse, Matcher: "", Command: "exit 1"},
	}
	r := NewHookRunner(hooks)

	err := r.Run(context.Background(), PostToolUse, "Edit", nil)
	if err != nil {
		t.Fatalf("PostToolUse failure should not return error, got: %v", err)
	}
}

func TestRun_EventEnvironmentVariables(t *testing.T) {
	// Write a script that dumps env vars to a temp file.
	dir := t.TempDir()
	outFile := filepath.Join(dir, "env.txt")

	cmd := `echo "event=$GLAMDRING_EVENT tool=$GLAMDRING_TOOL_NAME" > ` + outFile

	hooks := []Hook{
		{Event: SessionStart, Command: cmd},
	}
	r := NewHookRunner(hooks)

	err := r.Run(context.Background(), SessionStart, "N/A", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	got := strings.TrimSpace(string(data))
	want := "event=SessionStart tool=N/A"
	if got != want {
		t.Errorf("env vars:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestRun_StdinPassesToolInput(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "stdin.txt")

	cmd := "cat > " + outFile

	hooks := []Hook{
		{Event: PostToolUse, Matcher: "Write", Command: cmd},
	}
	r := NewHookRunner(hooks)

	input := json.RawMessage(`{"file_path":"/tmp/test.txt","content":"hello"}`)
	err := r.Run(context.Background(), PostToolUse, "Write", input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	if string(data) != string(input) {
		t.Errorf("stdin:\n  got:  %s\n  want: %s", data, input)
	}
}

func TestRun_MatcherRegex(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "ran.txt")

	hooks := []Hook{
		{Event: PostToolUse, Matcher: "Edit|Write", Command: "touch " + outFile},
	}
	r := NewHookRunner(hooks)

	// Should match "Edit".
	_ = r.Run(context.Background(), PostToolUse, "Edit", nil)
	if _, err := os.Stat(outFile); err != nil {
		t.Fatal("hook should have run for Edit")
	}
	os.Remove(outFile)

	// Should match "Write".
	_ = r.Run(context.Background(), PostToolUse, "Write", nil)
	if _, err := os.Stat(outFile); err != nil {
		t.Fatal("hook should have run for Write")
	}
	os.Remove(outFile)

	// Should NOT match "Read".
	_ = r.Run(context.Background(), PostToolUse, "Read", nil)
	if _, err := os.Stat(outFile); err == nil {
		t.Fatal("hook should NOT have run for Read")
	}
}

func TestRun_SessionEnd_FailureDoesNotBlock(t *testing.T) {
	hooks := []Hook{
		{Event: SessionEnd, Command: "exit 1"},
	}
	r := NewHookRunner(hooks)

	err := r.Run(context.Background(), SessionEnd, "", nil)
	if err != nil {
		t.Fatalf("SessionEnd failure should not return error, got: %v", err)
	}
}

func TestRun_Stop_FailureDoesNotBlock(t *testing.T) {
	hooks := []Hook{
		{Event: Stop, Command: "exit 1"},
	}
	r := NewHookRunner(hooks)

	err := r.Run(context.Background(), Stop, "", nil)
	if err != nil {
		t.Fatalf("Stop failure should not return error, got: %v", err)
	}
}

func TestRun_NoHooks(t *testing.T) {
	r := NewHookRunner(nil)
	err := r.Run(context.Background(), PreToolUse, "Bash", nil)
	if err != nil {
		t.Fatalf("unexpected error with no hooks: %v", err)
	}
}

func TestRun_PreToolUse_Success(t *testing.T) {
	hooks := []Hook{
		{Event: PreToolUse, Matcher: "Bash", Command: "true"},
	}
	r := NewHookRunner(hooks)

	err := r.Run(context.Background(), PreToolUse, "Bash", nil)
	if err != nil {
		t.Fatalf("successful PreToolUse hook should not error: %v", err)
	}
}

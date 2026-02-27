package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadDefaultLineLimit(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "large.txt")

	// Write a file with 3000 lines.
	var content strings.Builder
	for i := 1; i <= 3000; i++ {
		fmt.Fprintf(&content, "line %d\n", i)
	}
	if err := os.WriteFile(fpath, []byte(content.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := ReadTool{}
	input, _ := json.Marshal(readInput{FilePath: fpath})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Output, "truncated") {
		t.Error("expected truncation message for 3000-line file")
	}
	if !strings.Contains(result.Output, "showing first 2000 of 3001") {
		t.Errorf("expected 'showing first 2000 of 3001', got truncation msg in: %s", result.Output[len(result.Output)-100:])
	}

	// Verify we don't get line 2001+.
	if strings.Contains(result.Output, "line 2001") {
		t.Error("expected lines beyond 2000 to be truncated")
	}
}

func TestReadExplicitLimitNoTruncationMessage(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(fpath, []byte("a\nb\nc\nd\ne\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := ReadTool{}
	input, _ := json.Marshal(readInput{FilePath: fpath, Limit: 2})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result.Output, "truncated") {
		t.Error("should not show truncation message when user provides explicit limit")
	}
}

func TestReadLineTruncation(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "wide.txt")

	longLine := strings.Repeat("x", 3000)
	if err := os.WriteFile(fpath, []byte(longLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := ReadTool{}
	input, _ := json.Marshal(readInput{FilePath: fpath})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "line truncated") {
		t.Error("expected '(line truncated)' for long line")
	}
	// The line content should be capped at ~2000 chars + truncation message.
	lines := strings.Split(result.Output, "\n")
	if len(lines[0]) > 2200 { // allow overhead for line number prefix + truncation msg
		t.Errorf("line too long after truncation: %d chars", len(lines[0]))
	}
}

func TestReadTrackerRecording(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "tracked.txt")
	if err := os.WriteFile(fpath, []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tracker := NewReadTracker()
	tool := ReadTool{Tracker: tracker}
	input, _ := json.Marshal(readInput{FilePath: fpath})

	if tracker.HasRead(fpath) {
		t.Error("tracker should not have path before read")
	}

	_, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !tracker.HasRead(fpath) {
		t.Error("tracker should have path after successful read")
	}
}

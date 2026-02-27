package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteSafetyBlocksUnread(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "existing.txt")

	// Create an existing file.
	if err := os.WriteFile(fpath, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	tracker := NewReadTracker()
	tool := WriteTool{Tracker: tracker}

	input, _ := json.Marshal(writeInput{FilePath: fpath, Content: "overwrite"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when writing to unread existing file")
	}
	if !strings.Contains(result.Output, "not been read") {
		t.Errorf("expected 'not been read' in error, got %q", result.Output)
	}

	// Verify file was NOT overwritten.
	data, _ := os.ReadFile(fpath)
	if string(data) != "original" {
		t.Error("file should not have been overwritten")
	}
}

func TestWriteSafetyAllowsAfterRead(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(fpath, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	tracker := NewReadTracker()
	tracker.Record(fpath) // simulate a prior read

	tool := WriteTool{Tracker: tracker}
	input, _ := json.Marshal(writeInput{FilePath: fpath, Content: "updated"})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Output)
	}

	data, _ := os.ReadFile(fpath)
	if string(data) != "updated" {
		t.Errorf("expected file to be updated, got %q", string(data))
	}
}

func TestWriteSafetyAllowsNewFile(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "new.txt")

	tracker := NewReadTracker()
	tool := WriteTool{Tracker: tracker}

	input, _ := json.Marshal(writeInput{FilePath: fpath, Content: "new content"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error for new file: %s", result.Output)
	}

	data, _ := os.ReadFile(fpath)
	if string(data) != "new content" {
		t.Errorf("expected 'new content', got %q", string(data))
	}
}

func TestWriteNoTrackerAllowsAll(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(fpath, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := WriteTool{} // no tracker
	input, _ := json.Marshal(writeInput{FilePath: fpath, Content: "overwrite"})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("no tracker should allow all writes, got error: %s", result.Output)
	}
}

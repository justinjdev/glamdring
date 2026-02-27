package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditRejectNoOp(t *testing.T) {
	tool := EditTool{}
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(fpath, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	input, _ := json.Marshal(editInput{
		FilePath:  fpath,
		OldString: "hello",
		NewString: "hello",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for no-op edit")
	}
	if !strings.Contains(result.Output, "identical") {
		t.Errorf("expected 'identical' in error, got %q", result.Output)
	}
}

func TestEditPreservesPermissions(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "script.sh")
	if err := os.WriteFile(fpath, []byte("#!/bin/bash\necho hello"), 0o755); err != nil {
		t.Fatal(err)
	}

	tool := EditTool{}
	input, _ := json.Marshal(editInput{
		FilePath:  fpath,
		OldString: "hello",
		NewString: "world",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Output)
	}

	info, err := os.Stat(fpath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("expected permissions 0755, got %o", info.Mode().Perm())
	}

	data, _ := os.ReadFile(fpath)
	if !strings.Contains(string(data), "world") {
		t.Error("edit did not apply")
	}
}

func TestEditNormalOperation(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(fpath, []byte("foo bar baz"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := EditTool{}
	input, _ := json.Marshal(editInput{
		FilePath:  fpath,
		OldString: "bar",
		NewString: "qux",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Output)
	}

	data, _ := os.ReadFile(fpath)
	if string(data) != "foo qux baz" {
		t.Errorf("expected 'foo qux baz', got %q", string(data))
	}
}

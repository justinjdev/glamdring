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

func TestGlobResultLimit(t *testing.T) {
	dir := t.TempDir()

	// Create 50 files.
	for i := 0; i < 50; i++ {
		fpath := filepath.Join(dir, fmt.Sprintf("file%03d.txt", i))
		if err := os.WriteFile(fpath, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tool := GlobTool{CWD: dir}
	input, _ := json.Marshal(globInput{Pattern: "*.txt", Limit: 10})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(result.Output), "\n")
	// Filter out the truncation message line.
	var fileLines []string
	for _, line := range lines {
		if line != "" && !strings.HasPrefix(line, "...") {
			fileLines = append(fileLines, line)
		}
	}
	if len(fileLines) != 10 {
		t.Errorf("expected 10 results with limit=10, got %d", len(fileLines))
	}
	if !strings.Contains(result.Output, "showing 10 of") {
		t.Error("expected truncation message")
	}
}

func TestGlobSkipsNoiseDirectories(t *testing.T) {
	dir := t.TempDir()

	// Create files in normal and noise directories.
	normalDir := filepath.Join(dir, "src")
	nodeDir := filepath.Join(dir, "node_modules")
	os.MkdirAll(normalDir, 0o755)
	os.MkdirAll(nodeDir, 0o755)

	writeTestFile(t, filepath.Join(normalDir, "app.js"), "// app")
	writeTestFile(t, filepath.Join(nodeDir, "dep.js"), "// dep")

	tool := GlobTool{CWD: dir}
	input, _ := json.Marshal(globInput{Pattern: "**/*.js"})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Output, "app.js") {
		t.Error("expected app.js in results")
	}
	if strings.Contains(result.Output, "dep.js") {
		t.Error("expected node_modules/dep.js to be skipped")
	}
}

func TestGlobContextCancellation(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "test.txt"), "content")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tool := GlobTool{CWD: dir}
	input, _ := json.Marshal(globInput{Pattern: "*.txt"})
	result, _ := tool.Execute(ctx, input)
	// Should handle cancellation gracefully.
	_ = result
}

func TestGlobDefaultLimit(t *testing.T) {
	tool := GlobTool{CWD: "/tmp"}
	input, _ := json.Marshal(globInput{Pattern: "*.nonexistent"})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "no matches found" {
		t.Errorf("expected 'no matches found', got %q", result.Output)
	}
}

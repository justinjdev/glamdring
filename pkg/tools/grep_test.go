package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrepSchemaParams(t *testing.T) {
	tool := GrepTool{CWD: "/tmp"}

	// Create test files.
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "test.go"), "package main\nfunc Hello() {}\nfunc World() {}\n")

	// Test -A parameter.
	input := map[string]any{
		"pattern":     "Hello",
		"path":        dir,
		"output_mode": "content",
		"-A":          1,
	}
	raw, _ := json.Marshal(input)
	result, err := tool.Execute(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "World") {
		t.Errorf("expected -A context to include 'World', got %q", result.Output)
	}
}

func TestGrepCaseInsensitive(t *testing.T) {
	tool := GrepTool{}
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "test.txt"), "Hello World\nhello world\nHELLO WORLD\n")

	input := map[string]any{
		"pattern":     "hello",
		"path":        dir,
		"output_mode": "count",
		"-i":          true,
	}
	raw, _ := json.Marshal(input)
	result, err := tool.Execute(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, ":3") {
		t.Errorf("expected 3 case-insensitive matches, got %q", result.Output)
	}
}

func TestGrepLineNumbersDefault(t *testing.T) {
	tool := GrepTool{}
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "test.txt"), "alpha\nbeta\ngamma\n")

	input := map[string]any{
		"pattern":     "beta",
		"path":        dir,
		"output_mode": "content",
	}
	raw, _ := json.Marshal(input)
	result, err := tool.Execute(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "2:beta") {
		t.Errorf("expected line number in output, got %q", result.Output)
	}
}

func TestGrepLineNumbersDisabled(t *testing.T) {
	tool := GrepTool{}
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "test.txt"), "alpha\nbeta\ngamma\n")

	input := map[string]any{
		"pattern":     "beta",
		"path":        dir,
		"output_mode": "content",
		"-n":          false,
	}
	raw, _ := json.Marshal(input)
	result, err := tool.Execute(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result.Output, "2:beta") {
		t.Errorf("expected no line numbers when -n=false, got %q", result.Output)
	}
	if !strings.Contains(result.Output, "beta") {
		t.Errorf("expected 'beta' in output, got %q", result.Output)
	}
}

func TestGrepHeadLimitOffset(t *testing.T) {
	tool := GrepTool{}
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.txt"), "match1\n")
	writeTestFile(t, filepath.Join(dir, "b.txt"), "match2\n")
	writeTestFile(t, filepath.Join(dir, "c.txt"), "match3\n")
	writeTestFile(t, filepath.Join(dir, "d.txt"), "match4\n")

	input := map[string]any{
		"pattern":    "match",
		"path":       dir,
		"head_limit": 2,
		"offset":     1,
	}
	raw, _ := json.Marshal(input)
	result, err := tool.Execute(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(result.Output), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 results with head_limit=2 offset=1, got %d: %v", len(lines), lines)
	}
}

func TestGrepTypeFilter(t *testing.T) {
	tool := GrepTool{}
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "code.go"), "func main() {}\n")
	writeTestFile(t, filepath.Join(dir, "code.py"), "def main(): pass\n")
	writeTestFile(t, filepath.Join(dir, "code.js"), "function main() {}\n")

	input := map[string]any{
		"pattern": "main",
		"path":    dir,
		"type":    "go",
	}
	raw, _ := json.Marshal(input)
	result, err := tool.Execute(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "code.go") {
		t.Errorf("expected code.go in results, got %q", result.Output)
	}
	if strings.Contains(result.Output, "code.py") || strings.Contains(result.Output, "code.js") {
		t.Errorf("expected only .go files, got %q", result.Output)
	}
}

func TestGrepBinaryFileSkipped(t *testing.T) {
	tool := GrepTool{}
	dir := t.TempDir()

	// Write a binary file with null bytes.
	binaryContent := []byte("hello\x00world\n")
	if err := os.WriteFile(filepath.Join(dir, "binary.dat"), binaryContent, 0o644); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "text.txt"), "hello world\n")

	input := map[string]any{
		"pattern": "hello",
		"path":    dir,
	}
	raw, _ := json.Marshal(input)
	result, err := tool.Execute(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result.Output, "binary.dat") {
		t.Error("expected binary file to be skipped")
	}
	if !strings.Contains(result.Output, "text.txt") {
		t.Errorf("expected text.txt in results, got %q", result.Output)
	}
}

func TestGrepContextCancellation(t *testing.T) {
	tool := GrepTool{}
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "test.txt"), "match\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	input := map[string]any{
		"pattern": "match",
		"path":    dir,
	}
	raw, _ := json.Marshal(input)
	result, _ := tool.Execute(ctx, raw)
	// Should either return no matches or an error, not panic.
	_ = result
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

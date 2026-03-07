package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindInstructions_ProjectLevel_Claude(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.Mkdir(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# Project Instructions\nDo the thing."
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	deep := filepath.Join(root, "sub", "deep")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	proj, _, err := FindInstructions(deep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(proj, content) {
		t.Errorf("expected to contain %q, got %q", content, proj)
	}
}

func TestFindInstructions_ProjectLevel_Glamdring(t *testing.T) {
	root := t.TempDir()
	glamDir := filepath.Join(root, ".glamdring")
	if err := os.Mkdir(glamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# Glamdring Instructions"
	if err := os.WriteFile(filepath.Join(glamDir, "GLAMDRING.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	proj, _, err := FindInstructions(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(proj, content) {
		t.Errorf("expected to contain %q, got %q", content, proj)
	}
}

func TestFindInstructions_MixedNamespaces(t *testing.T) {
	root := t.TempDir()
	glamDir := filepath.Join(root, ".glamdring")
	claudeDir := filepath.Join(root, ".claude")
	if err := os.Mkdir(glamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(glamDir, "GLAMDRING.md"), []byte("glamdring content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("claude content"), 0o644); err != nil {
		t.Fatal(err)
	}

	proj, _, err := FindInstructions(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(proj, "glamdring content") {
		t.Errorf("expected 'glamdring content', got %q", proj)
	}
	if !strings.Contains(proj, "claude content") {
		t.Errorf("expected 'claude content', got %q", proj)
	}
	// Glamdring should appear before Claude.
	glamIdx := strings.Index(proj, "glamdring content")
	claudeIdx := strings.Index(proj, "claude content")
	if glamIdx > claudeIdx {
		t.Errorf("glamdring content should appear before claude content")
	}
}

func TestFindInstructions_NoneFound(t *testing.T) {
	dir := t.TempDir()
	proj, _, err := FindInstructions(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proj != "" {
		t.Errorf("expected empty project instructions, got %q", proj)
	}
}

func TestFindInstructions_CollectsAll(t *testing.T) {
	outer := t.TempDir()
	inner := filepath.Join(outer, "inner")

	outerClaude := filepath.Join(outer, ".claude")
	innerClaude := filepath.Join(inner, ".claude")
	if err := os.MkdirAll(outerClaude, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(innerClaude, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(outerClaude, "CLAUDE.md"), []byte("outer"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(innerClaude, "CLAUDE.md"), []byte("inner"), 0o644); err != nil {
		t.Fatal(err)
	}

	proj, _, err := FindInstructions(inner)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(proj, "inner") {
		t.Errorf("expected 'inner', got %q", proj)
	}
	if !strings.Contains(proj, "outer") {
		t.Errorf("expected 'outer', got %q", proj)
	}
}

func TestFindInstructions_BareGlamdringMD(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "GLAMDRING.md"), []byte("bare glamdring"), 0o644); err != nil {
		t.Fatal(err)
	}

	proj, _, err := FindInstructions(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(proj, "bare glamdring") {
		t.Errorf("expected bare GLAMDRING.md content, got %q", proj)
	}
}

func TestFindInstructions_BareCLAUDEMD(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("bare content"), 0o644); err != nil {
		t.Fatal(err)
	}

	proj, _, err := FindInstructions(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(proj, "bare content") {
		t.Errorf("expected bare CLAUDE.md content, got %q", proj)
	}
}

func TestFindInstructions_LocalMD(t *testing.T) {
	root := t.TempDir()
	glamDir := filepath.Join(root, ".glamdring")
	if err := os.Mkdir(glamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(glamDir, "GLAMDRING.md"), []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(glamDir, "GLAMDRING.local.md"), []byte("local overrides"), 0o644); err != nil {
		t.Fatal(err)
	}

	proj, _, err := FindInstructions(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(proj, "project") {
		t.Errorf("expected 'project', got %q", proj)
	}
	if !strings.Contains(proj, "local overrides") {
		t.Errorf("expected 'local overrides', got %q", proj)
	}
}

func TestFindProjectRoot_Glamdring(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".glamdring"), 0o755); err != nil {
		t.Fatal(err)
	}

	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	got := FindProjectRoot(deep)
	if got != root {
		t.Errorf("FindProjectRoot: got %q, want %q", got, root)
	}
}

func TestFindProjectRoot_Claude(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}

	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	got := FindProjectRoot(deep)
	if got != root {
		t.Errorf("FindProjectRoot: got %q, want %q", got, root)
	}
}

func TestFindProjectRoot_GlamdringPreferred(t *testing.T) {
	root := t.TempDir()
	// Both exist at the same level; .glamdring/ should be found first.
	if err := os.Mkdir(filepath.Join(root, ".glamdring"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := FindProjectRoot(root)
	if got != root {
		t.Errorf("FindProjectRoot: got %q, want %q", got, root)
	}
}

func TestFindProjectRoot_NotFound(t *testing.T) {
	dir := t.TempDir()
	got := FindProjectRoot(dir)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

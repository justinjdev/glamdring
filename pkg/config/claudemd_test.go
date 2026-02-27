package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindClaudeMD_ProjectLevel(t *testing.T) {
	// Create a temp directory tree: root/.claude/CLAUDE.md, root/sub/deep/
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

	proj, user, err := FindClaudeMD(deep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(proj, content) {
		t.Errorf("project CLAUDE.md: expected to contain %q, got %q", content, proj)
	}
	// User-level may or may not exist on the test machine; just ensure no error.
	_ = user
}

func TestFindClaudeMD_NoneFound(t *testing.T) {
	// Use a temp directory with no .claude/ at all.
	dir := t.TempDir()

	proj, _, err := FindClaudeMD(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proj != "" {
		t.Errorf("expected empty project CLAUDE.md, got %q", proj)
	}
}

func TestFindClaudeMD_CollectsAll(t *testing.T) {
	// Create two levels: outer/.claude/CLAUDE.md and outer/inner/.claude/CLAUDE.md
	// Both should be collected, inner first.
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

	proj, _, err := FindClaudeMD(inner)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(proj, "inner") {
		t.Errorf("expected project CLAUDE.md to contain 'inner', got %q", proj)
	}
	if !strings.Contains(proj, "outer") {
		t.Errorf("expected project CLAUDE.md to contain 'outer', got %q", proj)
	}
}

func TestFindClaudeMD_BareCLAUDEMD(t *testing.T) {
	root := t.TempDir()
	// Place a bare CLAUDE.md (not inside .claude/).
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("bare content"), 0o644); err != nil {
		t.Fatal(err)
	}

	proj, _, err := FindClaudeMD(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(proj, "bare content") {
		t.Errorf("expected bare CLAUDE.md content, got %q", proj)
	}
}

func TestFindClaudeMD_LocalMD(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.Mkdir(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.local.md"), []byte("local overrides"), 0o644); err != nil {
		t.Fatal(err)
	}

	proj, _, err := FindClaudeMD(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(proj, "project") {
		t.Errorf("expected 'project' in result, got %q", proj)
	}
	if !strings.Contains(proj, "local overrides") {
		t.Errorf("expected 'local overrides' in result, got %q", proj)
	}
}

func TestFindClaudeMD_BothBareAndNested(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.Mkdir(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("bare"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("nested"), 0o644); err != nil {
		t.Fatal(err)
	}

	proj, _, err := FindClaudeMD(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(proj, "bare") {
		t.Errorf("expected 'bare' in result, got %q", proj)
	}
	if !strings.Contains(proj, "nested") {
		t.Errorf("expected 'nested' in result, got %q", proj)
	}
}

func TestFindProjectRoot(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.Mkdir(claudeDir, 0o755); err != nil {
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

func TestFindProjectRoot_NotFound(t *testing.T) {
	dir := t.TempDir()
	got := FindProjectRoot(dir)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

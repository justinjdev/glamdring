package config

import (
	"os"
	"path/filepath"
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
	if proj != content {
		t.Errorf("project CLAUDE.md: got %q, want %q", proj, content)
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

func TestFindClaudeMD_StopsAtFirstFound(t *testing.T) {
	// Create two levels: outer/.claude/CLAUDE.md and outer/inner/.claude/CLAUDE.md
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

	// Start search from inner — should find inner first.
	proj, _, err := FindClaudeMD(inner)
	if err != nil {
		t.Fatal(err)
	}
	if proj != "inner" {
		t.Errorf("expected inner CLAUDE.md, got %q", proj)
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

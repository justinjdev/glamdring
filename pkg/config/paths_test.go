package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve_Primary(t *testing.T) {
	root := t.TempDir()
	glamDir := filepath.Join(root, ".glamdring")
	if err := os.Mkdir(glamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(glamDir, "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := Resolve(root, "config.json")
	want := filepath.Join(glamDir, "config.json")
	if got != want {
		t.Errorf("Resolve: got %q, want %q", got, want)
	}
}

func TestResolve_Fallback(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.Mkdir(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := Resolve(root, "settings.json")
	want := filepath.Join(claudeDir, "settings.json")
	if got != want {
		t.Errorf("Resolve: got %q, want %q", got, want)
	}
}

func TestResolve_PrimaryWins(t *testing.T) {
	root := t.TempDir()
	glamDir := filepath.Join(root, ".glamdring")
	claudeDir := filepath.Join(root, ".claude")
	if err := os.Mkdir(glamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(glamDir, "permissions.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "permissions.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := Resolve(root, "permissions.json")
	want := filepath.Join(glamDir, "permissions.json")
	if got != want {
		t.Errorf("Resolve: got %q, want %q (should prefer .glamdring/)", got, want)
	}
}

func TestResolve_NeitherExists(t *testing.T) {
	root := t.TempDir()
	got := Resolve(root, "config.json")
	if got != "" {
		t.Errorf("Resolve: expected empty string, got %q", got)
	}
}

func TestResolveDir_Primary(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".glamdring", "commands")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	got := ResolveDir(root, "commands")
	if got != dir {
		t.Errorf("ResolveDir: got %q, want %q", got, dir)
	}
}

func TestResolveDir_Fallback(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".claude", "commands")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	got := ResolveDir(root, "commands")
	if got != dir {
		t.Errorf("ResolveDir: got %q, want %q", got, dir)
	}
}

func TestResolveDir_FileNotDir(t *testing.T) {
	root := t.TempDir()
	glamDir := filepath.Join(root, ".glamdring")
	if err := os.Mkdir(glamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create "commands" as a file, not a directory.
	if err := os.WriteFile(filepath.Join(glamDir, "commands"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := ResolveDir(root, "commands")
	if got != "" {
		t.Errorf("ResolveDir: expected empty (file not dir), got %q", got)
	}
}

func TestResolveDir_NeitherExists(t *testing.T) {
	root := t.TempDir()
	got := ResolveDir(root, "commands")
	if got != "" {
		t.Errorf("ResolveDir: expected empty string, got %q", got)
	}
}

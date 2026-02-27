package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadHooks_NoFiles(t *testing.T) {
	dir := t.TempDir()
	hooks := LoadHooks(dir)
	if len(hooks) != 0 {
		t.Errorf("expected 0 hooks with no settings files, got %d", len(hooks))
	}
}

func TestLoadHooks_ProjectLevel(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.Mkdir(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settingsJSON := `{
		"hooks": [
			{"event": "PostToolUse", "matcher": "Edit|Write", "command": "echo edited"},
			{"event": "SessionStart", "command": "echo starting"}
		]
	}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settingsJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	hooks := LoadHooks(root)
	if len(hooks) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(hooks))
	}

	if hooks[0].Event != PostToolUse {
		t.Errorf("hook[0].Event: got %q, want PostToolUse", hooks[0].Event)
	}
	if hooks[0].Matcher != "Edit|Write" {
		t.Errorf("hook[0].Matcher: got %q, want 'Edit|Write'", hooks[0].Matcher)
	}
	if hooks[0].Command != "echo edited" {
		t.Errorf("hook[0].Command: got %q, want 'echo edited'", hooks[0].Command)
	}

	if hooks[1].Event != SessionStart {
		t.Errorf("hook[1].Event: got %q, want SessionStart", hooks[1].Event)
	}
	if hooks[1].Command != "echo starting" {
		t.Errorf("hook[1].Command: got %q, want 'echo starting'", hooks[1].Command)
	}
}

func TestLoadHooks_WalksUp(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.Mkdir(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settingsJSON := `{"hooks": [{"event": "Stop", "command": "echo bye"}]}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settingsJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	hooks := LoadHooks(deep)
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook from parent, got %d", len(hooks))
	}
	if hooks[0].Event != Stop {
		t.Errorf("hook.Event: got %q, want Stop", hooks[0].Event)
	}
}

func TestLoadHooks_MalformedJSON(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.Mkdir(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}

	hooks := LoadHooks(root)
	if len(hooks) != 0 {
		t.Errorf("expected 0 hooks on malformed JSON, got %d", len(hooks))
	}
}

func TestLoadHooks_NoHooksKey(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.Mkdir(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Valid JSON but no "hooks" key.
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{"model":"test"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	hooks := LoadHooks(root)
	if len(hooks) != 0 {
		t.Errorf("expected 0 hooks when key absent, got %d", len(hooks))
	}
}

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func intPtr(v int) *int { return &v }

func TestDefaultSettings(t *testing.T) {
	s := DefaultSettings()
	if s.Model != "claude-opus-4-6" {
		t.Errorf("default model: got %q, want %q", s.Model, "claude-opus-4-6")
	}
	if s.MaxTurns != nil {
		t.Errorf("default max turns: got %v, want nil", s.MaxTurns)
	}
}

func TestLoadSettings_NoFiles(t *testing.T) {
	dir := t.TempDir()
	s := LoadSettings(dir)
	if s.Model != "claude-opus-4-6" {
		t.Errorf("model: got %q, want default", s.Model)
	}
}

func TestLoadSettings_ProjectOverridesDefaults(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.Mkdir(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settings := Settings{
		Model:    "claude-sonnet-4-20250514",
		MaxTurns: intPtr(10),
	}
	data, _ := json.Marshal(settings)
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	s := LoadSettings(root)
	if s.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model: got %q, want claude-sonnet-4-20250514", s.Model)
	}
	if s.MaxTurns == nil || *s.MaxTurns != 10 {
		t.Errorf("max turns: got %v, want 10", s.MaxTurns)
	}
}

func TestLoadSettings_MCPServersMerge(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.Mkdir(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settings := Settings{
		MCPServers: map[string]MCPServerConfig{
			"myserver": {Command: "node", Args: []string{"server.js"}},
		},
	}
	data, _ := json.Marshal(settings)
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	s := LoadSettings(root)
	if s.MCPServers == nil {
		t.Fatal("expected MCPServers to be populated")
	}
	srv, ok := s.MCPServers["myserver"]
	if !ok {
		t.Fatal("expected 'myserver' in MCPServers")
	}
	if srv.Command != "node" {
		t.Errorf("command: got %q, want 'node'", srv.Command)
	}
	if len(srv.Args) != 1 || srv.Args[0] != "server.js" {
		t.Errorf("args: got %v, want [server.js]", srv.Args)
	}
}

func TestLoadSettings_MalformedJSON(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.Mkdir(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should not panic; falls back to defaults.
	s := LoadSettings(root)
	if s.Model != "claude-opus-4-6" {
		t.Errorf("expected defaults on malformed JSON, got model=%q", s.Model)
	}
}

func TestLoadSettings_WalksUp(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.Mkdir(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settings := Settings{Model: "custom-model"}
	data, _ := json.Marshal(settings)
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	deep := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	s := LoadSettings(deep)
	if s.Model != "custom-model" {
		t.Errorf("model: got %q, want 'custom-model'", s.Model)
	}
}

func TestMergeSettings(t *testing.T) {
	base := Settings{
		Model:    "default-model",
		MaxTurns: intPtr(5),
		MCPServers: map[string]MCPServerConfig{
			"existing": {Command: "python", Args: []string{"srv.py"}},
		},
	}

	override := Settings{
		Model: "override-model",
		MCPServers: map[string]MCPServerConfig{
			"new": {Command: "node", Args: []string{"index.js"}},
		},
	}

	mergeSettings(&base, &override)

	if base.Model != "override-model" {
		t.Errorf("model: got %q, want 'override-model'", base.Model)
	}
	// MaxTurns should remain since override is nil.
	if base.MaxTurns == nil || *base.MaxTurns != 5 {
		t.Errorf("max turns: got %v, want 5", base.MaxTurns)
	}
	// Both MCP servers should exist.
	if len(base.MCPServers) != 2 {
		t.Errorf("expected 2 MCP servers, got %d", len(base.MCPServers))
	}
	if _, ok := base.MCPServers["existing"]; !ok {
		t.Error("missing 'existing' server after merge")
	}
	if _, ok := base.MCPServers["new"]; !ok {
		t.Error("missing 'new' server after merge")
	}
}

func TestMergeSettings_ZeroMaxTurnsOverride(t *testing.T) {
	base := Settings{
		Model:    "default-model",
		MaxTurns: intPtr(5),
	}

	// Explicitly set MaxTurns to 0 (unlimited).
	override := Settings{
		MaxTurns: intPtr(0),
	}

	mergeSettings(&base, &override)

	if base.MaxTurns == nil {
		t.Fatal("expected MaxTurns to be set after merge")
	}
	if *base.MaxTurns != 0 {
		t.Errorf("max turns: got %d, want 0 (explicitly unlimited)", *base.MaxTurns)
	}
}

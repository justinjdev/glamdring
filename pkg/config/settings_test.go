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

func TestLoadSettings_MCPServerEnv(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.Mkdir(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settings := Settings{
		MCPServers: map[string]MCPServerConfig{
			"myserver": {
				Command: "node",
				Args:    []string{"server.js"},
				Env:     map[string]string{"API_KEY": "secret123"},
			},
		},
	}
	data, _ := json.Marshal(settings)
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	s := LoadSettings(root)
	srv := s.MCPServers["myserver"]
	if srv.Env == nil || srv.Env["API_KEY"] != "secret123" {
		t.Errorf("expected Env with API_KEY=secret123, got %v", srv.Env)
	}

	envSlice := srv.EnvSlice()
	if len(envSlice) != 1 || envSlice[0] != "API_KEY=secret123" {
		t.Errorf("EnvSlice() = %v, want [API_KEY=secret123]", envSlice)
	}
}

func TestLoadSettings_MCPToolsConfig(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.Mkdir(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settings := Settings{
		MCPServers: map[string]MCPServerConfig{
			"myserver": {
				Command: "node",
				Args:    []string{"server.js"},
				Tools: MCPToolsConfig{
					Enabled: []string{"read", "write"},
				},
			},
		},
	}
	data, _ := json.Marshal(settings)
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	s := LoadSettings(root)
	srv := s.MCPServers["myserver"]
	if len(srv.Tools.Enabled) != 2 {
		t.Errorf("expected 2 enabled tools, got %d", len(srv.Tools.Enabled))
	}
	if srv.Tools.Enabled[0] != "read" || srv.Tools.Enabled[1] != "write" {
		t.Errorf("expected [read, write], got %v", srv.Tools.Enabled)
	}
}

func TestEnvSlice_Empty(t *testing.T) {
	cfg := MCPServerConfig{Command: "echo"}
	if got := cfg.EnvSlice(); got != nil {
		t.Errorf("expected nil for empty Env, got %v", got)
	}
}

func TestMergeSettings_ExperimentalTeams(t *testing.T) {
	base := Settings{Model: "default-model"}
	override := Settings{Experimental: ExperimentalConfig{Teams: true}}

	mergeSettings(&base, &override)

	if !base.Experimental.Teams {
		t.Error("expected Experimental.Teams to be true after merge")
	}

	// Merging with false should not reset to false (only true is sticky).
	override2 := Settings{}
	mergeSettings(&base, &override2)
	if !base.Experimental.Teams {
		t.Error("expected Experimental.Teams to remain true after merge with zero value")
	}
}

func TestMergeSettings_Workflows(t *testing.T) {
	base := Settings{
		Model: "default-model",
		Workflows: map[string]WorkflowConfig{
			"existing": {Phases: []PhaseConfig{{Name: "plan", Tools: []string{"Read"}}}},
		},
	}
	override := Settings{
		Workflows: map[string]WorkflowConfig{
			"new": {Phases: []PhaseConfig{{Name: "work", Tools: []string{"Write"}}}},
		},
	}

	mergeSettings(&base, &override)

	if len(base.Workflows) != 2 {
		t.Fatalf("expected 2 workflows, got %d", len(base.Workflows))
	}
	if _, ok := base.Workflows["existing"]; !ok {
		t.Error("missing 'existing' workflow after merge")
	}
	if _, ok := base.Workflows["new"]; !ok {
		t.Error("missing 'new' workflow after merge")
	}
}

func TestLoadSettings_WorkflowValidation(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantOK  bool
	}{
		{
			name:   "valid workflow",
			json:   `{"workflows":{"custom":{"phases":[{"name":"work","tools":["Read","Write"]}]}}}`,
			wantOK: true,
		},
		{
			name:   "empty phases",
			json:   `{"workflows":{"bad":{"phases":[]}}}`,
			wantOK: false,
		},
		{
			name:   "duplicate phase names",
			json:   `{"workflows":{"bad":{"phases":[{"name":"a","tools":["Read"]},{"name":"a","tools":["Write"]}]}}}`,
			wantOK: false,
		},
		{
			name:   "phase with no tools",
			json:   `{"workflows":{"bad":{"phases":[{"name":"empty","tools":[]}]}}}`,
			wantOK: false,
		},
		{
			name:   "phase with no name",
			json:   `{"workflows":{"bad":{"phases":[{"tools":["Read"]}]}}}`,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			claudeDir := filepath.Join(root, ".claude")
			if err := os.Mkdir(claudeDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(tt.json), 0o644); err != nil {
				t.Fatal(err)
			}

			s := LoadSettings(root)
			if tt.wantOK {
				if len(s.Workflows) == 0 {
					t.Error("expected workflows to be loaded")
				}
			} else {
				// Invalid workflows cause fallback to defaults (no workflows).
				if len(s.Workflows) != 0 {
					t.Errorf("expected no workflows for invalid config, got %d", len(s.Workflows))
				}
			}
		})
	}
}

func TestValidateWorkflows_WarnsUnknownTools(t *testing.T) {
	// Unknown tools should produce warnings but NOT errors.
	wf := map[string]WorkflowConfig{
		"custom": {Phases: []PhaseConfig{
			{Name: "step", Tools: []string{"Read", "MagicTool"}},
		}},
	}
	err := validateWorkflows(wf)
	if err != nil {
		t.Errorf("expected no error for unknown tool (just a warning), got: %v", err)
	}
}

func TestValidateWorkflows_WarnsBadModelPattern(t *testing.T) {
	// Non-matching model names should produce warnings but NOT errors.
	wf := map[string]WorkflowConfig{
		"custom": {Phases: []PhaseConfig{
			{Name: "step", Tools: []string{"Read"}, Model: "gpt-4o"},
		}},
	}
	err := validateWorkflows(wf)
	if err != nil {
		t.Errorf("expected no error for bad model pattern (just a warning), got: %v", err)
	}
}

func TestValidateWorkflows_ValidModelPasses(t *testing.T) {
	wf := map[string]WorkflowConfig{
		"custom": {Phases: []PhaseConfig{
			{Name: "step", Tools: []string{"Read"}, Model: "claude-sonnet-4-6", Fallback: "claude-haiku-4-5-20251001"},
		}},
	}
	err := validateWorkflows(wf)
	if err != nil {
		t.Errorf("expected no error for valid config, got: %v", err)
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

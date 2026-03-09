package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
)

// Settings holds resolved configuration values.
type Settings struct {
	Model              string                     `json:"model,omitempty"`
	MaxTurns           *int                       `json:"max_turns,omitempty"`
	MCPServers         map[string]MCPServerConfig `json:"mcp_servers,omitempty"`
	Indexer            IndexerConfig              `json:"indexer,omitempty"`
	Experimental       ExperimentalConfig         `json:"experimental,omitempty"`
	Workflows          map[string]WorkflowConfig  `json:"workflows,omitempty"`
	Theme              string                     `json:"theme,omitempty"`
	HighContrast       bool                       `json:"high_contrast,omitempty"`
	Themes             map[string]UserThemeConfig `json:"themes,omitempty"`
	DisableUpdateCheck bool                       `json:"disable_update_check,omitempty"`
	ThinkingBudget     *int                       `json:"thinking_budget,omitempty"`
}

// UserThemeConfig holds user-defined theme colors from settings.json.
type UserThemeConfig struct {
	Bg        string `json:"bg"`
	Fg        string `json:"fg"`
	FgDim     string `json:"fg_dim"`
	FgBright  string `json:"fg_bright"`
	Primary   string `json:"primary"`
	Secondary string `json:"secondary"`
	Success   string `json:"success"`
	Error     string `json:"error"`
	Info      string `json:"info"`
	Subtle    string `json:"subtle"`
	Surface0  string `json:"surface0"`
	Surface1  string `json:"surface1"`
	Surface2  string `json:"surface2"`
}

// ExperimentalConfig holds flags for experimental features.
type ExperimentalConfig struct {
	Teams bool `json:"teams,omitempty"`
}

// WorkflowConfig defines a user-configurable workflow with named phases.
type WorkflowConfig struct {
	Phases []PhaseConfig `json:"phases"`
}

// PhaseConfig defines a single phase in a workflow.
type PhaseConfig struct {
	Name       string            `json:"name"`
	Tools      []string          `json:"tools"`
	Model      string            `json:"model,omitempty"`
	Fallback   string            `json:"fallback,omitempty"`
	Gate       string            `json:"gate,omitempty"`
	GateConfig map[string]string `json:"gate_config,omitempty"`
}

// IndexerConfig controls the shire code indexer integration.
type IndexerConfig struct {
	Enabled     *bool  `json:"enabled,omitempty"`      // nil = auto-detect, true = force on, false = disable
	Command     string `json:"command,omitempty"`      // indexer binary name (default: "shire")
	AutoRebuild *bool  `json:"auto_rebuild,omitempty"` // rebuild index after file-modifying turns (default: true)
	AutoBuild   *bool  `json:"auto_build,omitempty"`   // nil = prompt, true = auto-build, false = skip
}

// MCPServerConfig describes how to launch an MCP server process.
type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
	Tools   MCPToolsConfig    `json:"tools,omitempty"`
}

// MCPToolsConfig controls which tools are exposed from an MCP server.
// If Enabled is set, only those tools are registered (allowlist).
// If Disabled is set, those tools are excluded (denylist).
// Enabled takes precedence if both are set.
// Neither set = register all tools (default behavior).
type MCPToolsConfig struct {
	Enabled  []string `json:"enabled,omitempty"`
	Disabled []string `json:"disabled,omitempty"`
}

// EnvSlice converts the Env map to a slice of "KEY=VALUE" strings.
func (c MCPServerConfig) EnvSlice() []string {
	if len(c.Env) == 0 {
		return nil
	}
	out := make([]string, 0, len(c.Env))
	for k, v := range c.Env {
		out = append(out, k+"="+v)
	}
	return out
}

// DefaultSettings returns the baseline settings used when no config files
// are found.
func DefaultSettings() Settings {
	return Settings{
		Model: "claude-opus-4-6",
		// MaxTurns nil = unlimited (default).
	}
}

// LoadSettings loads settings from config files (.glamdring/config.json or
// .claude/settings.json). It checks both user-level and project-level
// settings, merging them with project taking precedence over user, and
// both taking precedence over defaults.
//
// If no settings files exist, defaults are returned (not an error).
func LoadSettings(cwd string) Settings {
	s := DefaultSettings()

	// User-level settings.
	if userSettings, ok := loadUserSettings(); ok {
		mergeSettings(&s, &userSettings)
	}

	// Project-level settings (overrides user).
	if projSettings, ok := loadProjectSettings(cwd); ok {
		mergeSettings(&s, &projSettings)
	}

	return s
}

// loadUserSettings reads config from the user-level config directory.
// Checks ~/.config/glamdring/config.json first, then ~/.claude/settings.json.
func loadUserSettings() (Settings, bool) {
	if path := ResolveUserFile(primaryConfigFile); path != "" {
		return loadSettingsFile(path)
	}
	if path := ResolveUserFile(fallbackConfigFile); path != "" {
		return loadSettingsFile(path)
	}
	return Settings{}, false
}

// loadProjectSettings walks up from cwd to find config.json or settings.json.
// Checks .glamdring/config.json then .claude/settings.json at each level.
func loadProjectSettings(cwd string) (Settings, bool) {
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return Settings{}, false
	}

	for {
		if path := Resolve(dir, primaryConfigFile); path != "" {
			if s, ok := loadSettingsFile(path); ok {
				return s, true
			}
		}
		if path := Resolve(dir, fallbackConfigFile); path != "" {
			if s, ok := loadSettingsFile(path); ok {
				return s, true
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return Settings{}, false
}

// loadSettingsFile reads and parses a single settings.json file.
func loadSettingsFile(path string) (Settings, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("warning: could not read settings file %s: %v", path, err)
		}
		return Settings{}, false
	}

	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		log.Printf("warning: failed to parse %s: %v", path, err)
		return Settings{}, false
	}
	if err := validateWorkflows(s.Workflows); err != nil {
		log.Printf("warning: invalid workflow in %s: %v", path, err)
		return Settings{}, false
	}
	return s, true
}

// SaveUserSetting updates a single key in the user-level config file
// (~/.config/glamdring/config.json). It reads the existing file, sets the
// key, and writes it back. Creates the file and directory if needed.
func SaveUserSetting(key string, value any) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}

	// Always write to the primary glamdring config to avoid polluting
	// fallback locations (e.g. ~/.claude/settings.json used by Claude Code).
	path := ResolveUserFile(primaryConfigFile)
	readPath := path
	if path == "" {
		// No primary config yet. If a legacy fallback config exists, seed
		// from it so we don't lose unrelated settings on the next load.
		if legacy := ResolveUserFile(fallbackConfigFile); legacy != "" {
			readPath = legacy
		}
		dir := filepath.Join(home, ".config", "glamdring")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create config dir: %w", err)
		}
		path = filepath.Join(dir, primaryConfigFile)
	}

	// Read existing config as raw JSON to preserve unknown fields.
	raw := make(map[string]json.RawMessage)
	if readPath != "" {
		if data, err := os.ReadFile(readPath); err == nil {
			if len(data) > 0 {
				if err := json.Unmarshal(data, &raw); err != nil {
					return fmt.Errorf("parse existing config %s: %w", readPath, err)
				}
			}
		}
	}

	val, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal value: %w", err)
	}
	raw[key] = val

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Write atomically via a temp file so a crash during write does not
	// corrupt the existing config.
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		// Clean up temp file if rename failed.
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(append(out, '\n')); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	return os.Rename(tmpPath, path)
}

// IndexerEnabled returns whether the indexer is enabled.
// nil (unset) means auto-detect; this helper resolves to explicit bool.
func (c IndexerConfig) IndexerEnabled() *bool { return c.Enabled }

// IndexerCommand returns the indexer binary name, defaulting to "shire".
func (c IndexerConfig) IndexerCommand() string {
	if c.Command != "" {
		return c.Command
	}
	return "shire"
}

// IndexerAutoRebuild returns whether auto-rebuild is enabled, defaulting to true.
func (c IndexerConfig) IndexerAutoRebuild() bool {
	if c.AutoRebuild != nil {
		return *c.AutoRebuild
	}
	return true
}

// IndexerAutoBuild returns the auto_build setting. nil = prompt user, true = build
// automatically without prompting, false = skip silently.
func (c IndexerConfig) IndexerAutoBuild() *bool { return c.AutoBuild }

// mergeSettings applies non-zero values from override onto base.
func mergeSettings(base, override *Settings) {
	if override.Model != "" {
		base.Model = override.Model
	}
	if override.MaxTurns != nil {
		base.MaxTurns = override.MaxTurns
	}
	if override.MCPServers != nil {
		if base.MCPServers == nil {
			base.MCPServers = make(map[string]MCPServerConfig)
		}
		for k, v := range override.MCPServers {
			base.MCPServers[k] = v
		}
	}
	if override.Indexer.Enabled != nil {
		base.Indexer.Enabled = override.Indexer.Enabled
	}
	if override.Indexer.Command != "" {
		base.Indexer.Command = override.Indexer.Command
	}
	if override.Indexer.AutoRebuild != nil {
		base.Indexer.AutoRebuild = override.Indexer.AutoRebuild
	}
	if override.Indexer.AutoBuild != nil {
		base.Indexer.AutoBuild = override.Indexer.AutoBuild
	}
	if override.Experimental.Teams {
		base.Experimental.Teams = true
	}
	if override.Workflows != nil {
		if base.Workflows == nil {
			base.Workflows = make(map[string]WorkflowConfig)
		}
		for k, v := range override.Workflows {
			base.Workflows[k] = v
		}
	}
	if override.Theme != "" {
		base.Theme = override.Theme
	}
	if override.HighContrast {
		base.HighContrast = true
	}
	if override.Themes != nil {
		if base.Themes == nil {
			base.Themes = make(map[string]UserThemeConfig)
		}
		for k, v := range override.Themes {
			base.Themes[k] = v
		}
	}
	if override.DisableUpdateCheck {
		base.DisableUpdateCheck = true
	}
	if override.ThinkingBudget != nil {
		base.ThinkingBudget = override.ThinkingBudget
	}
}

// knownToolNames is the set of built-in tool names recognized by glamdring.
var knownToolNames = map[string]bool{
	"Read": true, "Write": true, "Edit": true, "Bash": true,
	"Glob": true, "Grep": true,
}

// validModelPattern matches expected Claude model name formats.
var validModelPattern = regexp.MustCompile(`^claude-[a-z0-9-]+$`)

// validateWorkflows checks user-defined workflows for common configuration errors.
// Hard errors are returned for structural problems (empty phases, missing names, etc.).
// Warnings are logged for unrecognized tool names and unexpected model name formats.
func validateWorkflows(workflows map[string]WorkflowConfig) error {
	for name, wf := range workflows {
		if len(wf.Phases) == 0 {
			return fmt.Errorf("workflow %q has no phases", name)
		}
		seen := make(map[string]bool)
		for i, p := range wf.Phases {
			if p.Name == "" {
				return fmt.Errorf("workflow %q phase %d has no name", name, i)
			}
			if seen[p.Name] {
				return fmt.Errorf("workflow %q has duplicate phase name %q", name, p.Name)
			}
			seen[p.Name] = true
			if len(p.Tools) == 0 {
				return fmt.Errorf("workflow %q phase %q has no tools", name, p.Name)
			}
			for _, tool := range p.Tools {
				if !knownToolNames[tool] {
					log.Printf("warning: workflow %q phase %q references unknown tool %q", name, p.Name, tool)
				}
			}
			if p.Model != "" && !validModelPattern.MatchString(p.Model) {
				log.Printf("warning: workflow %q phase %q has unexpected model name %q", name, p.Name, p.Model)
			}
			if p.Fallback != "" && !validModelPattern.MatchString(p.Fallback) {
				log.Printf("warning: workflow %q phase %q has unexpected fallback model name %q", name, p.Name, p.Fallback)
			}
			if p.Gate != "" {
				switch p.Gate {
				case "auto", "leader", "condition":
					// valid
				default:
					return fmt.Errorf("workflow %q phase %q has unknown gate type %q; valid types: auto, leader, condition", name, p.Name, p.Gate)
				}
			}
			if p.Gate == "condition" && p.GateConfig["command"] == "" {
				return fmt.Errorf("workflow %q phase %q: condition gate requires gate_config.command", name, p.Name)
			}
		}
	}
	return nil
}

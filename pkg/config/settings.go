package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// Settings holds resolved configuration values.
type Settings struct {
	Model      string                      `json:"model,omitempty"`
	MaxTurns   int                         `json:"max_turns,omitempty"`
	MCPServers map[string]MCPServerConfig  `json:"mcp_servers,omitempty"`
	Indexer    IndexerConfig               `json:"indexer,omitempty"`
}

// IndexerConfig controls the shire code indexer integration.
type IndexerConfig struct {
	Enabled     *bool  `json:"enabled,omitempty"`      // nil = auto-detect, true = force on, false = disable
	Command     string `json:"command,omitempty"`       // indexer binary name (default: "shire")
	AutoRebuild *bool  `json:"auto_rebuild,omitempty"` // rebuild index after file-modifying turns (default: true)
}

// MCPServerConfig describes how to launch an MCP server process.
type MCPServerConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// DefaultSettings returns the baseline settings used when no config files
// are found.
func DefaultSettings() Settings {
	return Settings{
		Model:    "claude-opus-4-6",
		MaxTurns: 0, // unlimited
	}
}

// LoadSettings loads settings from .claude/settings.json if it exists.
// It checks both user-level (~/.claude/settings.json) and project-level
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

// loadUserSettings reads ~/.claude/settings.json.
func loadUserSettings() (Settings, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Settings{}, false
	}
	return loadSettingsFile(filepath.Join(home, ".claude", "settings.json"))
}

// loadProjectSettings walks up from cwd to find .claude/settings.json.
func loadProjectSettings(cwd string) (Settings, bool) {
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return Settings{}, false
	}

	for {
		candidate := filepath.Join(dir, ".claude", "settings.json")
		if s, ok := loadSettingsFile(candidate); ok {
			return s, true
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
		return Settings{}, false
	}

	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		log.Printf("warning: failed to parse %s: %v", path, err)
		return Settings{}, false
	}
	return s, true
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

// mergeSettings applies non-zero values from override onto base.
func mergeSettings(base, override *Settings) {
	if override.Model != "" {
		base.Model = override.Model
	}
	if override.MaxTurns != 0 {
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
}

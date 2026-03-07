package config

import (
	"os"
	"path/filepath"
)

const (
	primaryDir  = ".glamdring"
	fallbackDir = ".claude"

	primaryConfigFile  = "config.json"
	fallbackConfigFile = "settings.json"

	primaryInstructionsFile = "GLAMDRING.md"
	fallbackInstructionsFile = "CLAUDE.md"

	primaryLocalInstructionsFile  = "GLAMDRING.local.md"
	fallbackLocalInstructionsFile = "CLAUDE.local.md"
)

// Resolve checks .glamdring/<rel> then .claude/<rel> under baseDir,
// returning the first existing path. Returns empty string if neither exists.
func Resolve(baseDir, rel string) string {
	primary := filepath.Join(baseDir, primaryDir, rel)
	if _, err := os.Stat(primary); err == nil {
		return primary
	}
	fallback := filepath.Join(baseDir, fallbackDir, rel)
	if _, err := os.Stat(fallback); err == nil {
		return fallback
	}
	return ""
}

// ResolveDir checks .glamdring/<rel>/ then .claude/<rel>/ under baseDir,
// returning the first existing directory path. Returns empty string if neither exists.
func ResolveDir(baseDir, rel string) string {
	primary := filepath.Join(baseDir, primaryDir, rel)
	if info, err := os.Stat(primary); err == nil && info.IsDir() {
		return primary
	}
	fallback := filepath.Join(baseDir, fallbackDir, rel)
	if info, err := os.Stat(fallback); err == nil && info.IsDir() {
		return fallback
	}
	return ""
}

// UserConfigDir returns the user-level config directory.
// Checks ~/.config/glamdring/ first, then ~/.claude/.
func UserConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	primary := filepath.Join(home, ".config", "glamdring")
	if info, err := os.Stat(primary); err == nil && info.IsDir() {
		return primary
	}
	fallback := filepath.Join(home, fallbackDir)
	if info, err := os.Stat(fallback); err == nil && info.IsDir() {
		return fallback
	}
	return ""
}

// ResolveUserFile checks ~/.config/glamdring/<rel> then ~/.claude/<rel>,
// returning the first existing path. Returns empty string if neither exists.
func ResolveUserFile(rel string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	primary := filepath.Join(home, ".config", "glamdring", rel)
	if _, err := os.Stat(primary); err == nil {
		return primary
	}
	fallback := filepath.Join(home, fallbackDir, rel)
	if _, err := os.Stat(fallback); err == nil {
		return fallback
	}
	return ""
}

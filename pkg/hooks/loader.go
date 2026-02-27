package hooks

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// LoadHooks loads hook definitions from settings files.
// It checks both user-level (~/.claude/settings.json) and project-level
// (.claude/settings.json) configuration, combining hooks from both.
// Returns an empty slice if no hooks are configured.
func LoadHooks(cwd string) []Hook {
	var all []Hook

	// User-level hooks.
	if home, err := os.UserHomeDir(); err == nil {
		all = append(all, loadHooksFromFile(filepath.Join(home, ".claude", "settings.json"))...)
	}

	// Project-level hooks (walk up from cwd).
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return all
	}

	for {
		candidate := filepath.Join(dir, ".claude", "settings.json")
		all = append(all, loadHooksFromFile(candidate)...)

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return all
}

// loadHooksFromFile reads and parses the hooks array from a single settings file.
// The "hooks" value must be a JSON array of Hook objects; if the key is missing
// or holds a different type (e.g. an object), the file is silently skipped.
func loadHooksFromFile(path string) []Hook {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		log.Printf("warning: failed to parse %s: %v", path, err)
		return nil
	}

	hooksRaw, ok := raw["hooks"]
	if !ok || len(hooksRaw) == 0 {
		return nil
	}

	// Only decode if the value is a JSON array.
	if hooksRaw[0] != '[' {
		return nil
	}

	var hooks []Hook
	if err := json.Unmarshal(hooksRaw, &hooks); err != nil {
		log.Printf("warning: failed to parse hooks array in %s: %v", path, err)
		return nil
	}
	return hooks
}

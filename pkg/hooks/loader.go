package hooks

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/justin/glamdring/pkg/config"
)

// configFileNames lists the config files to check at each directory level,
// in priority order: glamdring config first, then claude settings fallback.
var configFileNames = []string{"config.json", "settings.json"}

// LoadHooks loads hook definitions from config files.
// It checks both user-level and project-level configuration, combining hooks
// from both. At each level, .glamdring/config.json is checked before
// .claude/settings.json. Returns an empty slice if no hooks are configured.
func LoadHooks(cwd string) []Hook {
	var all []Hook

	// User-level hooks.
	userDir := config.UserConfigDir()
	if userDir != "" {
		all = append(all, loadHooksFromDir(userDir)...)
	}

	// Project-level hooks (walk up from cwd).
	// Skip the user config directory since it was already loaded above.
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return all
	}

	visited := map[string]bool{}
	for {
		if !visited[dir] && dir != userDir {
			visited[dir] = true
			// Check .glamdring/ then .claude/ at this level.
			for _, cfgName := range configFileNames {
				if path := config.Resolve(dir, cfgName); path != "" {
					all = append(all, loadHooksFromFile(path)...)
					break // Use the first config file found at this level.
				}
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return all
}

// loadHooksFromDir checks for config files in a directory and loads hooks
// from the first one found.
func loadHooksFromDir(dir string) []Hook {
	for _, name := range configFileNames {
		path := filepath.Join(dir, name)
		if hooks := loadHooksFromFile(path); len(hooks) > 0 {
			return hooks
		}
	}
	return nil
}

// loadHooksFromFile reads and parses the hooks array from a single config file.
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

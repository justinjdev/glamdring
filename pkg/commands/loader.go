package commands

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/justin/glamdring/pkg/config"
)

// Command represents a slash command discovered from a .md file.
type Command struct {
	Name   string // e.g., "review" or "opsx new"
	Path   string // absolute path to the .md file
	Source string // "project" or "user"
}

// Discover scans .claude/commands/ directories for markdown files.
// Returns all found commands from both project and user level.
// Project commands take precedence over user commands with the same name.
func Discover(cwd string) []Command {
	var projectDir, userDir string

	if projectRoot := config.FindProjectRoot(cwd); projectRoot != "" {
		projectDir = filepath.Join(projectRoot, ".claude", "commands")
	}

	if home, err := os.UserHomeDir(); err == nil {
		userDir = filepath.Join(home, ".claude", "commands")
	}

	return discover(projectDir, userDir)
}

// discover scans the given project and user command directories.
// Either directory may be empty to skip scanning that level.
func discover(projectDir, userDir string) []Command {
	seen := make(map[string]bool)
	var commands []Command

	// Project-level commands first (they take precedence).
	if projectDir != "" {
		for _, cmd := range scanCommandDir(projectDir, "project") {
			seen[cmd.Name] = true
			commands = append(commands, cmd)
		}
	}

	// User-level commands.
	if userDir != "" {
		for _, cmd := range scanCommandDir(userDir, "user") {
			if !seen[cmd.Name] {
				commands = append(commands, cmd)
			}
		}
	}

	return commands
}

// scanCommandDir walks a directory for .md files and returns commands.
func scanCommandDir(dir string, source string) []Command {
	var commands []Command

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil
	}

	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip entries we can't read
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		// Derive command name from relative path, stripping the .md extension.
		// e.g., "opsx/new.md" -> "opsx new"
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}

		name := strings.TrimSuffix(rel, ".md")
		name = strings.ReplaceAll(name, string(filepath.Separator), " ")

		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}

		commands = append(commands, Command{
			Name:   name,
			Path:   abs,
			Source: source,
		})
		return nil
	})

	return commands
}

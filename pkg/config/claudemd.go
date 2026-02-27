package config

import (
	"os"
	"path/filepath"
)

// FindClaudeMD searches for CLAUDE.md files and returns their contents.
// Returns (projectLevel, userLevel string, err error).
//
// Project-level: starting from cwd, walk up the directory tree toward the
// filesystem root. At each directory, check for .claude/CLAUDE.md. Return
// the first one found.
//
// User-level: check ~/.claude/CLAUDE.md.
//
// If a file doesn't exist, the corresponding return value is empty string
// (not an error).
func FindClaudeMD(cwd string) (string, string, error) {
	projectLevel := findProjectClaudeMD(cwd)
	userLevel := findUserClaudeMD()
	return projectLevel, userLevel, nil
}

// findProjectClaudeMD walks up from dir looking for .claude/CLAUDE.md.
func findProjectClaudeMD(dir string) string {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}

	for {
		candidate := filepath.Join(dir, ".claude", "CLAUDE.md")
		data, err := os.ReadFile(candidate)
		if err == nil {
			return string(data)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root.
			break
		}
		dir = parent
	}
	return ""
}

// findUserClaudeMD reads ~/.claude/CLAUDE.md if it exists.
func findUserClaudeMD() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, ".claude", "CLAUDE.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

// FindProjectRoot walks up from cwd looking for a directory that contains
// .claude/. Returns the path if found, or empty string if not.
func FindProjectRoot(cwd string) string {
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return ""
	}

	for {
		candidate := filepath.Join(dir, ".claude")
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

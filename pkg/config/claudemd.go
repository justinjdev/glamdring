package config

import (
	"os"
	"path/filepath"
	"strings"
)

// FindInstructions searches for instruction files (GLAMDRING.md and CLAUDE.md)
// and returns their contents. Returns (projectLevel, userLevel string, err error).
//
// Project-level: starting from cwd, walk up the directory tree toward the
// filesystem root. At each directory, check for (in order):
//   - GLAMDRING.md (bare)
//   - .glamdring/GLAMDRING.md
//   - .glamdring/GLAMDRING.local.md
//   - CLAUDE.md (bare, fallback)
//   - .claude/CLAUDE.md (fallback)
//   - .claude/CLAUDE.local.md (fallback)
//
// All found files are concatenated (innermost directory first).
//
// User-level: check ~/.config/glamdring/GLAMDRING.md then ~/.claude/CLAUDE.md.
//
// If no files are found, the corresponding return value is empty string
// (not an error).
func FindInstructions(cwd string) (string, string, error) {
	projectLevel := findProjectInstructions(cwd)
	userLevel := findUserInstructions()
	return projectLevel, userLevel, nil
}

// findProjectInstructions walks up from dir collecting all instruction file
// variants. At each directory level it checks glamdring-namespaced files first,
// then claude-namespaced files. All found contents are concatenated.
func findProjectInstructions(dir string) string {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}

	var parts []string
	for {
		// Glamdring namespace (primary).
		if data, err := os.ReadFile(filepath.Join(dir, primaryInstructionsFile)); err == nil {
			parts = append(parts, string(data))
		}
		if data, err := os.ReadFile(filepath.Join(dir, primaryDir, primaryInstructionsFile)); err == nil {
			parts = append(parts, string(data))
		}
		if data, err := os.ReadFile(filepath.Join(dir, primaryDir, primaryLocalInstructionsFile)); err == nil {
			parts = append(parts, string(data))
		}

		// Claude namespace (fallback).
		if data, err := os.ReadFile(filepath.Join(dir, fallbackInstructionsFile)); err == nil {
			parts = append(parts, string(data))
		}
		if data, err := os.ReadFile(filepath.Join(dir, fallbackDir, fallbackInstructionsFile)); err == nil {
			parts = append(parts, string(data))
		}
		if data, err := os.ReadFile(filepath.Join(dir, fallbackDir, fallbackLocalInstructionsFile)); err == nil {
			parts = append(parts, string(data))
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return strings.Join(parts, "\n\n")
}

// findUserInstructions checks ~/.config/glamdring/GLAMDRING.md then
// ~/.claude/CLAUDE.md. Returns the concatenation of all found files.
func findUserInstructions() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	var parts []string
	if data, err := os.ReadFile(filepath.Join(home, ".config", "glamdring", primaryInstructionsFile)); err == nil {
		parts = append(parts, string(data))
	}
	if data, err := os.ReadFile(filepath.Join(home, fallbackDir, fallbackInstructionsFile)); err == nil {
		parts = append(parts, string(data))
	}
	return strings.Join(parts, "\n\n")
}

// FindProjectRoot walks up from cwd looking for a directory that contains
// .glamdring/ or .claude/. Returns the path if found, or empty string if not.
func FindProjectRoot(cwd string) string {
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return ""
	}

	for {
		if info, err := os.Stat(filepath.Join(dir, primaryDir)); err == nil && info.IsDir() {
			return dir
		}
		if info, err := os.Stat(filepath.Join(dir, fallbackDir)); err == nil && info.IsDir() {
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

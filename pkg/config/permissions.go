package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// PermissionConfig holds allow and deny rules loaded from .claude/permissions.json.
type PermissionConfig struct {
	Allow []PermissionRule `json:"allow"`
	Deny  []PermissionRule `json:"deny"`
}

// PermissionRule matches a tool invocation. Tool is required; Path and Command
// are optional filters that apply only when the tool is Read/Write/Edit (Path)
// or Bash (Command).
type PermissionRule struct {
	Tool    string `json:"tool"`
	Path    string `json:"path,omitempty"`
	Command string `json:"command,omitempty"`
}

// PermissionResult indicates whether a permission rule matched and what action
// to take.
type PermissionResult string

const (
	PermissionResultAllow   PermissionResult = "allow"
	PermissionResultDeny    PermissionResult = "deny"
	PermissionResultDefault PermissionResult = "default"
)

// LoadPermissions reads .claude/permissions.json from the project root.
// Returns nil if the file doesn't exist.
func LoadPermissions(cwd string) *PermissionConfig {
	path := filepath.Join(cwd, ".claude", "permissions.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var pc PermissionConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		return nil
	}
	return &pc
}

// Evaluate checks the deny and allow rules against a tool invocation.
// Deny rules are checked first; if any match, PermissionResultDeny is returned.
// Then allow rules; if any match, PermissionResultAllow is returned.
// Otherwise PermissionResultDefault is returned (fall through to normal prompt).
func (pc *PermissionConfig) Evaluate(toolName string, input map[string]any) PermissionResult {
	if pc == nil {
		return PermissionResultDefault
	}

	filePath := extractFilePath(input)
	command := extractCommand(input)

	for _, rule := range pc.Deny {
		if matchRule(rule, toolName, filePath, command) {
			return PermissionResultDeny
		}
	}
	for _, rule := range pc.Allow {
		if matchRule(rule, toolName, filePath, command) {
			return PermissionResultAllow
		}
	}
	return PermissionResultDefault
}

// matchRule checks whether a single rule matches the given tool invocation.
func matchRule(rule PermissionRule, toolName, filePath, command string) bool {
	if rule.Tool != toolName {
		return false
	}
	if rule.Path != "" && !matchGlobPattern(rule.Path, filePath) {
		return false
	}
	if rule.Command != "" && !matchGlobPattern(rule.Command, command) {
		return false
	}
	return true
}

// matchGlobPattern handles simple glob matching:
//   - "prefix*" matches strings starting with prefix
//   - "dir/**" matches strings starting with dir/ (recursive)
//   - Otherwise falls back to filepath.Match
func matchGlobPattern(pattern, value string) bool {
	if value == "" {
		return false
	}

	// Handle "dir/**" -- recursive directory match.
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return strings.HasPrefix(value, prefix+"/") || value == prefix
	}

	// Handle "prefix*" -- simple prefix match.
	if strings.HasSuffix(pattern, "*") && !strings.Contains(pattern[:len(pattern)-1], "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(value, prefix)
	}

	// Fall back to filepath.Match.
	matched, err := filepath.Match(pattern, value)
	if err != nil {
		return false
	}
	return matched
}

// extractFilePath pulls the file_path field from tool input (Read, Write, Edit).
func extractFilePath(input map[string]any) string {
	if v, ok := input["file_path"].(string); ok {
		return v
	}
	return ""
}

// extractCommand pulls the command field from tool input (Bash).
func extractCommand(input map[string]any) string {
	if v, ok := input["command"].(string); ok {
		return v
	}
	return ""
}

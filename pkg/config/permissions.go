package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// PermissionConfig holds allow and deny rules loaded from permissions.json
// (.glamdring/ or .claude/ namespace).
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

// LoadPermissions reads permissions.json from the project config directory.
// Checks .glamdring/permissions.json first, then .claude/permissions.json.
// Returns (nil, nil) if no permissions file exists. Returns an error if the
// file exists but cannot be read or contains invalid JSON.
func LoadPermissions(cwd string) (*PermissionConfig, error) {
	path := Resolve(cwd, "permissions.json")
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var pc PermissionConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if err := pc.validate(); err != nil {
		return nil, fmt.Errorf("validating %s: %w", path, err)
	}
	return &pc, nil
}

// validate checks that all rules have a non-empty Tool and valid glob patterns.
func (pc *PermissionConfig) validate() error {
	for i, rule := range pc.Deny {
		if err := validateRule(rule); err != nil {
			return fmt.Errorf("deny rule %d: %w", i, err)
		}
	}
	for i, rule := range pc.Allow {
		if err := validateRule(rule); err != nil {
			return fmt.Errorf("allow rule %d: %w", i, err)
		}
	}
	return nil
}

// validateRule checks a single permission rule for structural validity.
func validateRule(rule PermissionRule) error {
	if rule.Tool == "" {
		return fmt.Errorf("tool is required")
	}
	if rule.Path != "" {
		if err := validatePattern(rule.Path); err != nil {
			return fmt.Errorf("invalid path pattern %q: %w", rule.Path, err)
		}
	}
	if rule.Command != "" {
		if err := validatePattern(rule.Command); err != nil {
			return fmt.Errorf("invalid command pattern %q: %w", rule.Command, err)
		}
	}
	return nil
}

// validatePattern checks that a glob pattern is syntactically valid.
func validatePattern(pattern string) error {
	// Our custom patterns (dir/** and prefix*) are always valid.
	if strings.HasSuffix(pattern, "/**") {
		return nil
	}
	if strings.HasSuffix(pattern, "*") && !strings.Contains(pattern[:len(pattern)-1], "*") {
		return nil
	}
	// Validate against filepath.Match with a dummy value.
	_, err := filepath.Match(pattern, "test")
	return err
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
	if rule.Path != "" && !MatchGlobPattern(rule.Path, filePath) {
		return false
	}
	if rule.Command != "" && !MatchGlobPattern(rule.Command, command) {
		return false
	}
	return true
}

// MatchGlobPattern handles simple glob matching:
//   - "prefix*" matches strings starting with prefix
//   - "dir/**" matches strings starting with dir/ (recursive)
//   - Otherwise falls back to filepath.Match
//
func MatchGlobPattern(pattern, value string) bool {
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
		log.Printf("warning: invalid glob pattern %q: %v", pattern, err)
		return false
	}
	return matched
}

// TeamScope defines path restrictions for team agents. If set, file-modifying tools
// are checked against the scope before normal permission evaluation.
type TeamScope struct {
	AllowPatterns []string
	DenyPatterns  []string
}

// EvaluateTeamScope checks whether a tool invocation is permitted under a team scope.
// Returns PermissionResultDeny if the scope forbids the operation, or
// PermissionResultDefault to fall through to normal permission evaluation.
// Only file-modifying tools (Write, Edit) are scope-checked.
func EvaluateTeamScope(scope *TeamScope, toolName string, input map[string]any) PermissionResult {
	if scope == nil {
		return PermissionResultDefault
	}
	// Only enforce scope on file-modifying tools.
	if toolName != "Write" && toolName != "Edit" {
		return PermissionResultDefault
	}
	filePath := extractFilePath(input)
	if filePath == "" {
		return PermissionResultDefault
	}

	// Check deny patterns first.
	for _, pattern := range scope.DenyPatterns {
		if MatchGlobPattern(pattern, filePath) {
			return PermissionResultDeny
		}
	}

	// If allow patterns are specified, the path must match at least one.
	if len(scope.AllowPatterns) > 0 {
		for _, pattern := range scope.AllowPatterns {
			if MatchGlobPattern(pattern, filePath) {
				return PermissionResultDefault // allowed, continue to normal eval
			}
		}
		return PermissionResultDeny // no allow pattern matched
	}

	return PermissionResultDefault
}

// extractFilePath pulls the file_path field from tool input (Read, Write, Edit).
// The path is cleaned with filepath.Clean to normalize traversal sequences
// (e.g., /tmp/../etc/passwd -> /etc/passwd) and prevent deny rule bypasses.
func extractFilePath(input map[string]any) string {
	if v, ok := input["file_path"].(string); ok {
		return filepath.Clean(v)
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

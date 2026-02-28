package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEvaluate_NilConfig(t *testing.T) {
	var pc *PermissionConfig
	result := pc.Evaluate("Bash", map[string]any{"command": "ls"})
	if result != PermissionResultDefault {
		t.Errorf("expected default, got %s", result)
	}
}

func TestEvaluate_AllowTool(t *testing.T) {
	pc := &PermissionConfig{
		Allow: []PermissionRule{{Tool: "Write"}},
	}
	result := pc.Evaluate("Write", map[string]any{"file_path": "/tmp/foo"})
	if result != PermissionResultAllow {
		t.Errorf("expected allow, got %s", result)
	}
}

func TestEvaluate_DenyOverridesAllow(t *testing.T) {
	pc := &PermissionConfig{
		Allow: []PermissionRule{{Tool: "Bash"}},
		Deny:  []PermissionRule{{Tool: "Bash", Command: "rm *"}},
	}
	result := pc.Evaluate("Bash", map[string]any{"command": "rm -rf /"})
	// Command "rm -rf /" does not match "rm *" via matchGlobPattern (prefix "rm ").
	// But "rm *" with * suffix means prefix match "rm ", and "rm -rf /" starts with "rm ".
	if result != PermissionResultDeny {
		t.Errorf("expected deny, got %s", result)
	}
}

func TestEvaluate_DenyExactCommand(t *testing.T) {
	pc := &PermissionConfig{
		Allow: []PermissionRule{{Tool: "Bash"}},
		Deny:  []PermissionRule{{Tool: "Bash", Command: "rm -rf /"}},
	}
	result := pc.Evaluate("Bash", map[string]any{"command": "rm -rf /"})
	if result != PermissionResultDeny {
		t.Errorf("expected deny, got %s", result)
	}
}

func TestEvaluate_AllowWithPath(t *testing.T) {
	pc := &PermissionConfig{
		Allow: []PermissionRule{{Tool: "Write", Path: "/tmp/**"}},
	}
	result := pc.Evaluate("Write", map[string]any{"file_path": "/tmp/foo/bar.txt"})
	if result != PermissionResultAllow {
		t.Errorf("expected allow, got %s", result)
	}
}

func TestEvaluate_AllowPathNoMatch(t *testing.T) {
	pc := &PermissionConfig{
		Allow: []PermissionRule{{Tool: "Write", Path: "/tmp/**"}},
	}
	result := pc.Evaluate("Write", map[string]any{"file_path": "/etc/passwd"})
	if result != PermissionResultDefault {
		t.Errorf("expected default (path doesn't match), got %s", result)
	}
}

func TestEvaluate_DenyPath(t *testing.T) {
	pc := &PermissionConfig{
		Allow: []PermissionRule{{Tool: "Write"}},
		Deny:  []PermissionRule{{Tool: "Write", Path: "/etc/**"}},
	}
	result := pc.Evaluate("Write", map[string]any{"file_path": "/etc/passwd"})
	if result != PermissionResultDeny {
		t.Errorf("expected deny, got %s", result)
	}
}

func TestEvaluate_NoMatchingTool(t *testing.T) {
	pc := &PermissionConfig{
		Allow: []PermissionRule{{Tool: "Write"}},
	}
	result := pc.Evaluate("Bash", map[string]any{"command": "ls"})
	if result != PermissionResultDefault {
		t.Errorf("expected default, got %s", result)
	}
}

func TestEvaluate_CommandPrefixMatch(t *testing.T) {
	pc := &PermissionConfig{
		Allow: []PermissionRule{{Tool: "Bash", Command: "go *"}},
	}
	result := pc.Evaluate("Bash", map[string]any{"command": "go test ./..."})
	if result != PermissionResultAllow {
		t.Errorf("expected allow, got %s", result)
	}
}

func TestMatchGlobPattern_RecursiveDir(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{"src/**", "src/main.go", true},
		{"src/**", "src/pkg/foo.go", true},
		{"src/**", "src", true},
		{"src/**", "other/main.go", false},
		{"src/**", "", false},
	}
	for _, tt := range tests {
		got := matchGlobPattern(tt.pattern, tt.value)
		if got != tt.want {
			t.Errorf("matchGlobPattern(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
		}
	}
}

func TestMatchGlobPattern_PrefixStar(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{"go *", "go test", true},
		{"go *", "go build ./...", true},
		{"go *", "npm install", false},
		{"go *", "", false},
	}
	for _, tt := range tests {
		got := matchGlobPattern(tt.pattern, tt.value)
		if got != tt.want {
			t.Errorf("matchGlobPattern(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
		}
	}
}

func TestLoadPermissions_FileExists(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	os.MkdirAll(claudeDir, 0o755)

	data := `{"allow":[{"tool":"Bash","command":"go *"}],"deny":[{"tool":"Write","path":"/etc/**"}]}`
	os.WriteFile(filepath.Join(claudeDir, "permissions.json"), []byte(data), 0o644)

	pc := LoadPermissions(dir)
	if pc == nil {
		t.Fatal("expected non-nil PermissionConfig")
	}
	if len(pc.Allow) != 1 || pc.Allow[0].Tool != "Bash" {
		t.Errorf("unexpected allow rules: %+v", pc.Allow)
	}
	if len(pc.Deny) != 1 || pc.Deny[0].Tool != "Write" {
		t.Errorf("unexpected deny rules: %+v", pc.Deny)
	}
}

func TestLoadPermissions_NoFile(t *testing.T) {
	pc := LoadPermissions(t.TempDir())
	if pc != nil {
		t.Error("expected nil when file doesn't exist")
	}
}

func TestLoadPermissions_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	os.WriteFile(filepath.Join(claudeDir, "permissions.json"), []byte("{invalid"), 0o644)

	pc := LoadPermissions(dir)
	if pc != nil {
		t.Error("expected nil for invalid JSON")
	}
}

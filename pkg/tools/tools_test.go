package tools

import "testing"

func TestDefaultToolsPascalCaseNames(t *testing.T) {
	allTools := DefaultTools("/tmp")

	// All tool names should be PascalCase and match the alwaysAllowTools keys
	// or permission-required tool names used throughout the codebase.
	expected := map[string]bool{
		"Read":  true,
		"Write": true,
		"Edit":  true,
		"Bash":  true,
		"Glob":  true,
		"Grep":  true,
	}

	if len(allTools) != len(expected) {
		t.Fatalf("expected %d default tools, got %d", len(expected), len(allTools))
	}

	for _, tool := range allTools {
		name := tool.Name()
		if !expected[name] {
			t.Errorf("unexpected tool name %q; expected PascalCase name in %v", name, expected)
		}
		// Verify first character is uppercase.
		if name[0] < 'A' || name[0] > 'Z' {
			t.Errorf("tool name %q does not start with uppercase letter", name)
		}
	}
}

func TestTaskToolName(t *testing.T) {
	tt := &TaskTool{}
	if tt.Name() != "Task" {
		t.Errorf("TaskTool.Name() = %q, want %q", tt.Name(), "Task")
	}
}

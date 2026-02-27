package tools

import "testing"

func TestDefaultToolsPascalCaseNames(t *testing.T) {
	allTools := DefaultTools("/tmp")

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

func TestDefaultToolsShareTracker(t *testing.T) {
	allTools := DefaultTools("/tmp")

	var readTool *ReadTool
	var writeTool *WriteTool
	for _, tool := range allTools {
		switch v := tool.(type) {
		case ReadTool:
			readTool = &v
		case WriteTool:
			writeTool = &v
		}
	}

	if readTool == nil {
		t.Fatal("ReadTool not found in default tools")
	}
	if writeTool == nil {
		t.Fatal("WriteTool not found in default tools")
	}
	if readTool.Tracker == nil {
		t.Fatal("ReadTool.Tracker is nil")
	}
	if writeTool.Tracker == nil {
		t.Fatal("WriteTool.Tracker is nil")
	}
	if readTool.Tracker != writeTool.Tracker {
		t.Error("ReadTool and WriteTool do not share the same tracker")
	}
}

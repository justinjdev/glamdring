package agents

import (
	"testing"
)

func TestNewRegistry_Get(t *testing.T) {
	reg := NewRegistry([]AgentDefinition{
		{Name: "reviewer", Description: "Reviews code", Prompt: "Review.", Tools: []string{"Read"}},
		{Name: "writer", Description: "Writes code", Prompt: "Write.", Tools: []string{"Bash"}},
	})

	a, ok := reg.Get("reviewer")
	if !ok {
		t.Fatal("expected to find 'reviewer'")
	}
	if a.Description != "Reviews code" {
		t.Errorf("description: got %q, want %q", a.Description, "Reviews code")
	}

	_, ok = reg.Get("nonexistent")
	if ok {
		t.Error("expected 'nonexistent' to not be found")
	}
}

func TestNewRegistry_FirstEntryWins(t *testing.T) {
	reg := NewRegistry([]AgentDefinition{
		{Name: "reviewer", Description: "Project reviewer", Prompt: "Project."},
		{Name: "reviewer", Description: "User reviewer", Prompt: "User."},
	})

	a, ok := reg.Get("reviewer")
	if !ok {
		t.Fatal("expected to find 'reviewer'")
	}
	if a.Description != "Project reviewer" {
		t.Errorf("description: got %q, want %q (first entry should win)", a.Description, "Project reviewer")
	}
}

func TestRegistry_Names(t *testing.T) {
	reg := NewRegistry([]AgentDefinition{
		{Name: "writer"},
		{Name: "reviewer"},
		{Name: "auditor"},
	})

	names := reg.Names()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}

	// Should be sorted.
	want := []string{"auditor", "reviewer", "writer"}
	for i, name := range names {
		if name != want[i] {
			t.Errorf("names[%d]: got %q, want %q", i, name, want[i])
		}
	}
}

func TestRegistry_Empty(t *testing.T) {
	reg := NewRegistry(nil)
	if names := reg.Names(); len(names) != 0 {
		t.Errorf("expected 0 names, got %d", len(names))
	}
	_, ok := reg.Get("anything")
	if ok {
		t.Error("expected empty registry to return false")
	}
}

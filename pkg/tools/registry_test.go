package tools

import (
	"context"
	"encoding/json"
	"testing"
)

// stubTool is a minimal Tool implementation for testing.
type stubTool struct {
	name string
}

func (s *stubTool) Name() string                                                   { return s.name }
func (s *stubTool) Description() string                                            { return "stub" }
func (s *stubTool) Schema() json.RawMessage                                        { return json.RawMessage(`{}`) }
func (s *stubTool) Execute(_ context.Context, _ json.RawMessage) (Result, error) { return Result{}, nil }

func TestRegistryDeterministicOrder(t *testing.T) {
	r := NewRegistry()
	names := []string{"Alpha", "Beta", "Gamma", "Delta"}

	for _, name := range names {
		r.Register(&stubTool{name: name})
	}

	// Run multiple times to verify determinism.
	for iter := 0; iter < 20; iter++ {
		all := r.All()
		if len(all) != len(names) {
			t.Fatalf("iter %d: expected %d tools, got %d", iter, len(names), len(all))
		}
		for i, tool := range all {
			if tool.Name() != names[i] {
				t.Errorf("iter %d: position %d: expected %q, got %q", iter, i, names[i], tool.Name())
			}
		}
	}
}

func TestRegistrySchemasOrder(t *testing.T) {
	r := NewRegistry()
	names := []string{"First", "Second", "Third"}

	for _, name := range names {
		r.Register(&stubTool{name: name})
	}

	for iter := 0; iter < 20; iter++ {
		schemas := r.Schemas()
		if len(schemas) != len(names) {
			t.Fatalf("expected %d schemas, got %d", len(names), len(schemas))
		}
		for i, schema := range schemas {
			var m map[string]any
			if err := json.Unmarshal(schema, &m); err != nil {
				t.Fatalf("failed to unmarshal schema: %v", err)
			}
			if m["name"] != names[i] {
				t.Errorf("iter %d: schema %d: expected name %q, got %q", iter, i, names[i], m["name"])
			}
		}
	}
}

func TestRegistryReRegisterPreservesOrder(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "A"})
	r.Register(&stubTool{name: "B"})
	r.Register(&stubTool{name: "C"})

	// Re-register B — should not duplicate it.
	r.Register(&stubTool{name: "B"})

	all := r.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(all))
	}

	expected := []string{"A", "B", "C"}
	for i, tool := range all {
		if tool.Name() != expected[i] {
			t.Errorf("position %d: expected %q, got %q", i, expected[i], tool.Name())
		}
	}
}

package teams

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/justin/glamdring/pkg/tools"
)

// mockPhaseTracker is a simple in-test PhaseTracker implementation.
type mockPhaseTracker struct {
	phases map[string][]Phase
	index  map[string]int
}

func newMockPhaseTracker() *mockPhaseTracker {
	return &mockPhaseTracker{
		phases: make(map[string][]Phase),
		index:  make(map[string]int),
	}
}

func (m *mockPhaseTracker) SetPhases(agentName string, phases []Phase) {
	m.phases[agentName] = phases
	m.index[agentName] = 0
}

func (m *mockPhaseTracker) Current(agentName string) (*Phase, int, error) {
	phases, ok := m.phases[agentName]
	if !ok || len(phases) == 0 {
		return nil, -1, fmt.Errorf("no phases configured for %q", agentName)
	}
	idx := m.index[agentName]
	if idx >= len(phases) {
		return nil, -1, fmt.Errorf("all phases completed for %q", agentName)
	}
	return &phases[idx], idx, nil
}

func (m *mockPhaseTracker) Advance(agentName string) (*Phase, error) {
	phases, ok := m.phases[agentName]
	if !ok {
		return nil, fmt.Errorf("no phases configured for %q", agentName)
	}
	m.index[agentName]++
	idx := m.index[agentName]
	if idx >= len(phases) {
		return nil, fmt.Errorf("no more phases for %q", agentName)
	}
	return &phases[idx], nil
}

func (m *mockPhaseTracker) AdvanceTo(agentName string, phaseName string) (*Phase, error) {
	phases, ok := m.phases[agentName]
	if !ok {
		return nil, fmt.Errorf("no phases configured for %q", agentName)
	}
	for i, p := range phases {
		if p.Name == phaseName {
			m.index[agentName] = i
			return &phases[i], nil
		}
	}
	return nil, fmt.Errorf("phase %q not found for %q", phaseName, agentName)
}

func (m *mockPhaseTracker) Remove(agentName string) {
	delete(m.phases, agentName)
	delete(m.index, agentName)
}

// registryStubTool is a stub for use with tools.Registry.
type registryStubTool struct {
	name string
}

func (s *registryStubTool) Name() string            { return s.name }
func (s *registryStubTool) Description() string     { return "stub: " + s.name }
func (s *registryStubTool) Schema() json.RawMessage { return json.RawMessage(`{}`) }
func (s *registryStubTool) Execute(_ context.Context, _ json.RawMessage) (tools.Result, error) {
	return tools.Result{Output: s.name + " executed"}, nil
}

func newTestRegistry(names ...string) *tools.Registry {
	r := tools.NewRegistry()
	for _, name := range names {
		r.Register(&registryStubTool{name: name})
	}
	return r
}

func TestPhaseRegistry_ResearchPhaseExcludesWrite(t *testing.T) {
	reg := newTestRegistry("Read", "Glob", "Grep", "Write", "Edit", "Bash", "TaskUpdate", "SendMessage")
	tracker := newMockPhaseTracker()
	tracker.SetPhases("agent-a", []Phase{
		{Name: "research", Tools: []string{"Bash"}},
		{Name: "implement", Tools: []string{"Write", "Edit", "Bash"}},
	})

	pr := NewPhaseRegistry(reg, tracker, "agent-a",
		[]string{"TaskUpdate", "SendMessage"},
		[]string{"Read", "Glob", "Grep"},
	)

	// In research phase: Write and Edit should not be available.
	schemas := pr.Schemas()
	schemaNames := extractNames(t, schemas)

	if _, ok := schemaNames["Write"]; ok {
		t.Error("Write should not be in research phase schemas")
	}
	if _, ok := schemaNames["Edit"]; ok {
		t.Error("Edit should not be in research phase schemas")
	}

	// Read tools should be available.
	for _, name := range []string{"Read", "Glob", "Grep"} {
		if _, ok := schemaNames[name]; !ok {
			t.Errorf("%s should be in research phase schemas", name)
		}
	}

	// Bash should be available (in phase tools).
	if _, ok := schemaNames["Bash"]; !ok {
		t.Error("Bash should be in research phase schemas")
	}

	// Get should return nil for Write.
	if pr.Get("Write") != nil {
		t.Error("Get(Write) should return nil in research phase")
	}

	// Execute should fail for Write.
	result, err := pr.Execute(context.Background(), "Write", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("Execute(Write) should fail in research phase")
	}
}

func TestPhaseRegistry_ImplementPhaseIncludesWrite(t *testing.T) {
	reg := newTestRegistry("Read", "Glob", "Grep", "Write", "Edit", "Bash", "TaskUpdate")
	tracker := newMockPhaseTracker()
	tracker.SetPhases("agent-a", []Phase{
		{Name: "research", Tools: []string{"Bash"}},
		{Name: "implement", Tools: []string{"Write", "Edit", "Bash"}},
	})

	// Advance to implement phase.
	tracker.Advance("agent-a")

	pr := NewPhaseRegistry(reg, tracker, "agent-a",
		[]string{"TaskUpdate"},
		[]string{"Read", "Glob", "Grep"},
	)

	schemas := pr.Schemas()
	schemaNames := extractNames(t, schemas)

	for _, name := range []string{"Write", "Edit", "Bash", "Read", "Glob", "Grep", "TaskUpdate"} {
		if _, ok := schemaNames[name]; !ok {
			t.Errorf("%s should be available in implement phase", name)
		}
	}

	if pr.Get("Write") == nil {
		t.Error("Get(Write) should return the tool in implement phase")
	}

	result, err := pr.Execute(context.Background(), "Write", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("Execute(Write) should succeed in implement phase, got: %s", result.Output)
	}
}

func TestPhaseRegistry_TeamToolsAlwaysAvailable(t *testing.T) {
	reg := newTestRegistry("Read", "Write", "TaskUpdate", "SendMessage")
	tracker := newMockPhaseTracker()
	tracker.SetPhases("agent-a", []Phase{
		{Name: "research", Tools: []string{"Read"}},
	})

	pr := NewPhaseRegistry(reg, tracker, "agent-a",
		[]string{"TaskUpdate", "SendMessage"},
		nil,
	)

	schemas := pr.Schemas()
	schemaNames := extractNames(t, schemas)

	for _, name := range []string{"TaskUpdate", "SendMessage"} {
		if _, ok := schemaNames[name]; !ok {
			t.Errorf("team tool %s should always be available", name)
		}
	}
}

func TestPhaseRegistry_AdvanceChangesTools(t *testing.T) {
	reg := newTestRegistry("Read", "Write", "Edit", "Bash")
	tracker := newMockPhaseTracker()
	tracker.SetPhases("agent-a", []Phase{
		{Name: "research", Tools: []string{"Bash"}},
		{Name: "implement", Tools: []string{"Write", "Edit", "Bash"}},
	})

	pr := NewPhaseRegistry(reg, tracker, "agent-a", []string{}, []string{"Read"})

	// Research phase: Write not available.
	if pr.Get("Write") != nil {
		t.Error("Write should not be available in research phase")
	}

	// Advance to implement phase.
	tracker.Advance("agent-a")

	// Now Write should be available.
	if pr.Get("Write") == nil {
		t.Error("Write should be available after advancing to implement phase")
	}
}

func TestPhaseRegistry_NoPhasesAllToolsAvailable(t *testing.T) {
	reg := newTestRegistry("Read", "Write", "Edit", "Bash", "Grep")
	tracker := newMockPhaseTracker()
	// No phases configured for this agent.

	pr := NewPhaseRegistry(reg, tracker, "agent-a", []string{}, []string{})

	// All tools from the base registry should be available.
	schemas := pr.Schemas()
	if len(schemas) != 5 {
		t.Errorf("expected 5 schemas, got %d", len(schemas))
	}

	for _, name := range []string{"Read", "Write", "Edit", "Bash", "Grep"} {
		if pr.Get(name) == nil {
			t.Errorf("Get(%s) should return tool when no phases configured", name)
		}
	}

	result, err := pr.Execute(context.Background(), "Write", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("Execute(Write) should succeed when no phases configured, got: %s", result.Output)
	}
}

func TestPhaseRegistry_ExecuteStreamingFiltered(t *testing.T) {
	reg := newTestRegistry("Read", "Write")
	tracker := newMockPhaseTracker()
	tracker.SetPhases("agent-a", []Phase{
		{Name: "research", Tools: []string{"Read"}},
	})

	pr := NewPhaseRegistry(reg, tracker, "agent-a", []string{}, []string{})

	result, err := pr.ExecuteStreaming(context.Background(), "Write", json.RawMessage(`{}`), func(s string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("ExecuteStreaming(Write) should fail in research phase")
	}
}

// extractNames parses schema JSON and returns a set of tool names.
func extractNames(t *testing.T, schemas []json.RawMessage) map[string]bool {
	t.Helper()
	names := make(map[string]bool)
	for _, schema := range schemas {
		var m map[string]any
		if err := json.Unmarshal(schema, &m); err != nil {
			t.Fatalf("failed to unmarshal schema: %v", err)
		}
		if name, ok := m["name"].(string); ok {
			names[name] = true
		}
	}
	return names
}

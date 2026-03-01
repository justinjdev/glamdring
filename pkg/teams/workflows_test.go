package teams

import "testing"

func TestWorkflowRPIV(t *testing.T) {
	phases := WorkflowRPIV()
	if len(phases) != 4 {
		t.Fatalf("expected 4 phases, got %d", len(phases))
	}
	expected := []string{"research", "plan", "implement", "verify"}
	for i, p := range phases {
		if p.Name != expected[i] {
			t.Errorf("phase %d: expected %q, got %q", i, expected[i], p.Name)
		}
	}

	// Plan phase should have leader gate; others should default to auto (empty).
	if phases[1].Gate != "leader" {
		t.Errorf("plan phase gate: expected %q, got %q", "leader", phases[1].Gate)
	}
	for _, i := range []int{0, 2, 3} {
		if phases[i].Gate != "" {
			t.Errorf("phase %q gate: expected empty (auto), got %q", phases[i].Name, phases[i].Gate)
		}
	}
}

func TestWorkflowPlanImplement(t *testing.T) {
	phases := WorkflowPlanImplement()
	if len(phases) != 2 {
		t.Fatalf("expected 2 phases, got %d", len(phases))
	}
	if phases[0].Name != "plan" {
		t.Errorf("expected 'plan', got %q", phases[0].Name)
	}
	if phases[1].Name != "implement" {
		t.Errorf("expected 'implement', got %q", phases[1].Name)
	}

	// Plan phase should have leader gate.
	if phases[0].Gate != "leader" {
		t.Errorf("plan phase gate: expected %q, got %q", "leader", phases[0].Gate)
	}
	if phases[1].Gate != "" {
		t.Errorf("implement phase gate: expected empty (auto), got %q", phases[1].Gate)
	}
}

func TestWorkflowScopedOnly(t *testing.T) {
	phases := WorkflowScopedOnly()
	if len(phases) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(phases))
	}
	if phases[0].Name != "work" {
		t.Errorf("expected 'work', got %q", phases[0].Name)
	}
}

func TestWorkflowNone(t *testing.T) {
	phases := WorkflowNone()
	if phases != nil {
		t.Errorf("expected nil, got %v", phases)
	}
}

func TestResolveWorkflow(t *testing.T) {
	tests := []struct {
		name      string
		workflow  string
		custom    []Phase
		expected  int // expected number of phases
		expectErr bool
	}{
		{name: "rpiv", workflow: "rpiv", expected: 4},
		{name: "plan-implement", workflow: "plan-implement", expected: 2},
		{name: "scoped", workflow: "scoped", expected: 1},
		{name: "none", workflow: "none", expected: 0},
		{name: "empty string", workflow: "", expected: 0},
		{name: "unknown", workflow: "unknown", expectErr: true},
		{name: "custom overrides", workflow: "rpiv", custom: []Phase{{Name: "custom"}}, expected: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			phases, err := ResolveWorkflow(tc.workflow, tc.custom, nil)
			if tc.expectErr {
				if err == nil {
					t.Error("expected error for unknown workflow")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := len(phases)
			if got != tc.expected {
				t.Errorf("expected %d phases, got %d", tc.expected, got)
			}
		})
	}
}

func TestResolveWorkflow_CustomOverridesName(t *testing.T) {
	custom := []Phase{{Name: "my-phase", Tools: []string{"Read"}}}
	phases, err := ResolveWorkflow("rpiv", custom, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(phases) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(phases))
	}
	if phases[0].Name != "my-phase" {
		t.Errorf("expected 'my-phase', got %q", phases[0].Name)
	}
}

func TestResolveWorkflow_UnknownReturnsError(t *testing.T) {
	_, err := ResolveWorkflow("nonexistent", nil, nil)
	if err == nil {
		t.Error("expected error for unknown workflow name")
	}
}

func TestResolveWorkflow_RegisteredWorkflow(t *testing.T) {
	registered := map[string][]Phase{
		"my-workflow": {
			{Name: "analyze", Tools: []string{"Read", "Grep"}, Model: "claude-sonnet-4-6"},
			{Name: "execute", Tools: []string{"Write", "Edit"}, Model: "claude-opus-4-6"},
		},
	}
	phases, err := ResolveWorkflow("my-workflow", nil, registered)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(phases) != 2 {
		t.Fatalf("expected 2 phases, got %d", len(phases))
	}
	if phases[0].Name != "analyze" {
		t.Errorf("expected 'analyze', got %q", phases[0].Name)
	}
	if phases[1].Name != "execute" {
		t.Errorf("expected 'execute', got %q", phases[1].Name)
	}
}

func TestResolveWorkflow_RegisteredOverridesBuiltin(t *testing.T) {
	registered := map[string][]Phase{
		"rpiv": {
			{Name: "custom-research", Tools: []string{"Read"}, Model: "claude-sonnet-4-6"},
		},
	}
	phases, err := ResolveWorkflow("rpiv", nil, registered)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(phases) != 1 {
		t.Fatalf("expected 1 phase (registered override), got %d", len(phases))
	}
	if phases[0].Name != "custom-research" {
		t.Errorf("expected 'custom-research', got %q", phases[0].Name)
	}
}

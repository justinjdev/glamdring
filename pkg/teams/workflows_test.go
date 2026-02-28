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
		name     string
		workflow string
		custom   []Phase
		expected int // expected number of phases
	}{
		{name: "rpiv", workflow: "rpiv", expected: 4},
		{name: "plan-implement", workflow: "plan-implement", expected: 2},
		{name: "scoped", workflow: "scoped", expected: 1},
		{name: "none", workflow: "none", expected: 0},
		{name: "empty string", workflow: "", expected: 0},
		{name: "unknown", workflow: "unknown", expected: 0},
		{name: "custom overrides", workflow: "rpiv", custom: []Phase{{Name: "custom"}}, expected: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			phases := ResolveWorkflow(tc.workflow, tc.custom)
			got := len(phases)
			if got != tc.expected {
				t.Errorf("expected %d phases, got %d", tc.expected, got)
			}
		})
	}
}

func TestResolveWorkflow_CustomOverridesName(t *testing.T) {
	custom := []Phase{{Name: "my-phase", Tools: []string{"Read"}}}
	phases := ResolveWorkflow("rpiv", custom)

	if len(phases) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(phases))
	}
	if phases[0].Name != "my-phase" {
		t.Errorf("expected 'my-phase', got %q", phases[0].Name)
	}
}

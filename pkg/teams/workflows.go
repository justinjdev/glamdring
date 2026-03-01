package teams

import "fmt"

// WorkflowRPIV returns the Research-Plan-Implement-Verify workflow
// with tiered models: lighter models for research/planning, heavier for implementation.
func WorkflowRPIV() []Phase {
	return []Phase{
		{Name: "research", Tools: []string{"Read", "Glob", "Grep", "Bash"}, Model: "claude-haiku-4-5-20251001", Fallback: "claude-sonnet-4-6"},
		{Name: "plan", Tools: []string{"Read", "Glob", "Grep"}, Model: "claude-sonnet-4-6"},
		{Name: "implement", Tools: []string{"Read", "Write", "Edit", "Bash", "Glob", "Grep"}, Model: "claude-opus-4-6", Fallback: "claude-sonnet-4-6"},
		{Name: "verify", Tools: []string{"Read", "Bash", "Glob", "Grep"}, Model: "claude-sonnet-4-6"},
	}
}

// WorkflowPlanImplement returns a simpler two-phase workflow.
func WorkflowPlanImplement() []Phase {
	return []Phase{
		{Name: "plan", Tools: []string{"Read", "Glob", "Grep"}, Model: "claude-sonnet-4-6"},
		{Name: "implement", Tools: []string{"Read", "Write", "Edit", "Bash", "Glob", "Grep"}, Model: "claude-opus-4-6", Fallback: "claude-sonnet-4-6"},
	}
}

// WorkflowScopedOnly returns a single-phase workflow with all tools but file scoping enforced.
func WorkflowScopedOnly() []Phase {
	return []Phase{
		{Name: "work", Tools: []string{"Read", "Write", "Edit", "Bash", "Glob", "Grep"}, Model: "claude-sonnet-4-6"},
	}
}

// WorkflowNone returns an empty phase list (no workflow enforcement).
func WorkflowNone() []Phase {
	return nil
}

// ResolveWorkflow returns phases for a named built-in workflow, or custom phases if provided.
// Returns an error for unknown workflow names.
func ResolveWorkflow(name string, custom []Phase) ([]Phase, error) {
	if len(custom) > 0 {
		return custom, nil
	}
	switch name {
	case "rpiv":
		return WorkflowRPIV(), nil
	case "plan-implement":
		return WorkflowPlanImplement(), nil
	case "scoped":
		return WorkflowScopedOnly(), nil
	case "", "none":
		return WorkflowNone(), nil
	default:
		return nil, fmt.Errorf("unknown workflow %q", name)
	}
}

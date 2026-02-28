package teams

// WorkflowRPIV returns the Research-Plan-Implement-Verify workflow.
func WorkflowRPIV() []Phase {
	return []Phase{
		{Name: "research", Tools: []string{"Read", "Glob", "Grep", "Bash"}, Model: "claude-sonnet-4-6"},
		{Name: "plan", Tools: []string{"Read", "Glob", "Grep"}, Model: "claude-sonnet-4-6"},
		{Name: "implement", Tools: []string{"Read", "Write", "Edit", "Bash", "Glob", "Grep"}, Model: "claude-sonnet-4-6"},
		{Name: "verify", Tools: []string{"Read", "Bash", "Glob", "Grep"}, Model: "claude-sonnet-4-6"},
	}
}

// WorkflowPlanImplement returns a simpler two-phase workflow.
func WorkflowPlanImplement() []Phase {
	return []Phase{
		{Name: "plan", Tools: []string{"Read", "Glob", "Grep"}, Model: "claude-sonnet-4-6"},
		{Name: "implement", Tools: []string{"Read", "Write", "Edit", "Bash", "Glob", "Grep"}, Model: "claude-sonnet-4-6"},
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
func ResolveWorkflow(name string, custom []Phase) []Phase {
	if len(custom) > 0 {
		return custom
	}
	switch name {
	case "rpiv":
		return WorkflowRPIV()
	case "plan-implement":
		return WorkflowPlanImplement()
	case "scoped":
		return WorkflowScopedOnly()
	case "", "none":
		return WorkflowNone()
	default:
		return WorkflowNone()
	}
}

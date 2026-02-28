package hooks

// Event identifies a point in the agent lifecycle where hooks can fire.
type Event string

const (
	PreToolUse       Event = "PreToolUse"
	PostToolUse      Event = "PostToolUse"
	SessionStart     Event = "SessionStart"
	SessionEnd       Event = "SessionEnd"
	Stop             Event = "Stop"
	ContextThreshold Event = "ContextThreshold"
)

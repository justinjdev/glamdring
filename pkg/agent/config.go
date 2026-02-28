package agent

import (
	"github.com/justin/glamdring/pkg/auth"
	"github.com/justin/glamdring/pkg/config"
	"github.com/justin/glamdring/pkg/hooks"
	"github.com/justin/glamdring/pkg/tools"
)

// Config holds the configuration for an agent run.
type Config struct {
	Prompt       string
	Model        string
	Creds        auth.Credentials
	SystemPrompt string
	Tools        []tools.Tool
	MaxTurns     *int
	CWD          string
	HookRunner   *hooks.HookRunner
	Permissions  *config.PermissionConfig
	Yolo         bool

	// TeamState is opaque state for team agents. When non-nil, it indicates
	// this agent is a team member and enables message channel injection.
	TeamState any

	// PriorityMessages delivers high-priority inter-agent messages (shutdown,
	// approval) that are injected between tool executions.
	PriorityMessages <-chan any

	// RegularMessages delivers normal inter-agent messages that are injected
	// between turns (before the next API call).
	RegularMessages <-chan any

	// ToolProvider optionally overrides the tool registry with a custom
	// provider (e.g., PhaseRegistry for team agents). If nil, a standard
	// Registry is built from Tools.
	ToolProvider tools.ToolProvider

	// TeamScope restricts file-modifying tools to specific path patterns.
	// When set, operations outside the scope are denied before normal
	// permission evaluation.
	TeamScope *config.TeamScope
}

// DefaultModel is the default Claude model to use.
const DefaultModel = "claude-opus-4-6"

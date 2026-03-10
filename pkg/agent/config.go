package agent

import (
	"context"

	"github.com/justin/glamdring/pkg/auth"
	"github.com/justin/glamdring/pkg/config"
	"github.com/justin/glamdring/pkg/hooks"
	"github.com/justin/glamdring/pkg/session"
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

	// CancelFunc allows external callers (e.g., force shutdown) to cancel
	// the agent's context, terminating the agentic loop.
	CancelFunc context.CancelFunc

	// PhaseTransitionCallback is called after a workflow phase change is
	// detected in syncPhaseModel. The session passes its current messages
	// so the callback can trigger compaction or context archiving.
	// v1 limitation: inject_context conflicts across phases are not resolved;
	// callers should be aware that archived context may include stale
	// inject_context blocks from prior phases.
	PhaseTransitionCallback func(messages []string)

	// TeamScope restricts file-modifying tools to specific path patterns.
	// When set, operations outside the scope are denied before normal
	// permission evaluation.
	TeamScope *config.TeamScope

	// Store is the session persistence store. When non-nil, conversation
	// history is automatically saved to JSONL files on each turn.
	Store *session.Store

	// Endpoint overrides the API endpoint URL. Intended for testing with
	// httptest servers.
	Endpoint string

	// ThinkingBudget overrides the extended thinking token budget. nil = use
	// the model default (adaptive for claude-*-4-6, 10000 tokens for older
	// supported models), a pointer to 0 disables thinking entirely, and any
	// positive value sets a custom budget (non-adaptive models only).
	ThinkingBudget *int
}

// DefaultModel is the default Claude model to use.
const DefaultModel = "claude-opus-4-6"

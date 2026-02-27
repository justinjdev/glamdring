package agent

import (
	"github.com/justin/glamdring/pkg/hooks"
	"github.com/justin/glamdring/pkg/tools"
)

// Config holds the configuration for an agent run.
type Config struct {
	Prompt       string
	Model        string
	APIKey       string
	SystemPrompt string
	Tools        []tools.Tool
	MaxTurns     int
	CWD          string
	HookRunner   *hooks.HookRunner
}

// DefaultModel is the default Claude model to use.
const DefaultModel = "claude-opus-4-6"

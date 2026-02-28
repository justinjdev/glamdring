package tools

import (
	"context"
	"encoding/json"
)

// ToolProvider abstracts over tool registries, allowing the agent loop
// to work with different tool backends (e.g., Registry for standalone
// agents, PhaseRegistry for team agents).
type ToolProvider interface {
	Schemas() []json.RawMessage
	Get(name string) Tool
	Execute(ctx context.Context, name string, input json.RawMessage) (Result, error)
	ExecuteStreaming(ctx context.Context, name string, input json.RawMessage, onOutput func(string)) (Result, error)
}

// PhaseModelProvider is an optional interface that ToolProvider implementations
// can satisfy to expose the current phase's model and fallback model. The agent
// session uses this to switch models on phase advance and to fall back on API errors.
type PhaseModelProvider interface {
	CurrentPhaseModel() (model string, fallback string)
}

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

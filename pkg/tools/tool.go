package tools

import (
	"context"
	"encoding/json"
)

// Tool is the interface that all tools (built-in and MCP) must implement.
type Tool interface {
	Name() string
	Description() string
	Schema() json.RawMessage
	Execute(ctx context.Context, input json.RawMessage) (Result, error)
}

// Result is the output of a tool execution.
type Result struct {
	Output  string
	IsError bool
}

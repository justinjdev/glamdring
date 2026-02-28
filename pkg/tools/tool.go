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

// StreamingTool is an optional interface for tools that can stream output
// incrementally. Tools that implement this get their output displayed in
// real time rather than all at once when execution completes.
//
// The onOutput callback has the following contract:
//   - It may be nil; implementations must check before calling.
//   - Calls must be synchronous (from the calling goroutine or serialized).
//   - Callers must not block inside the callback.
//   - The callback will not be invoked concurrently by the framework.
type StreamingTool interface {
	Tool
	ExecuteStreaming(ctx context.Context, input json.RawMessage, onOutput func(string)) (Result, error)
}

// Result is the output of a tool execution.
type Result struct {
	Output  string
	IsError bool
}

package mcp

import (
	"context"
	"encoding/json"

	"github.com/justin/glamdring/pkg/tools"
)

// MCPTool wraps a single MCP tool definition so it implements the tools.Tool
// interface. Tool calls are forwarded to the owning Client.
type MCPTool struct {
	client      *Client
	name        string // qualified: "server_tool"
	mcpName     string // original MCP tool name
	description string
	schema      json.RawMessage
}

// NewMCPTool creates a tool adapter for the given MCP tool definition.
func NewMCPTool(client *Client, serverName string, def ToolDefinition) *MCPTool {
	// Prefix with server name to namespace tools across servers.
	qualifiedName := serverName + "_" + def.Name
	return &MCPTool{
		client:      client,
		name:        qualifiedName,
		mcpName:     def.Name,
		description: def.Description,
		schema:      def.InputSchema,
	}
}

func (t *MCPTool) Name() string              { return t.name }
func (t *MCPTool) Description() string        { return t.description }
func (t *MCPTool) Schema() json.RawMessage    { return t.schema }

// MCPToolName returns the original (unqualified) tool name used in MCP calls.
func (t *MCPTool) MCPToolName() string {
	return t.mcpName
}

func (t *MCPTool) Execute(ctx context.Context, input json.RawMessage) (tools.Result, error) {
	text, isError, err := t.client.CallTool(ctx, t.MCPToolName(), input)
	if err != nil {
		return tools.Result{Output: err.Error(), IsError: true}, nil
	}
	return tools.Result{Output: text, IsError: isError}, nil
}

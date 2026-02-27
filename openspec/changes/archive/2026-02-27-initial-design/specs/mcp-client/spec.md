## ADDED Requirements

### Requirement: Connect to MCP servers via stdio
The system SHALL spawn MCP server processes and communicate via JSON-RPC over stdin/stdout. The system SHALL send `initialize`, `tools/list`, and `tools/call` requests per the MCP protocol specification.

#### Scenario: Discover tools from MCP server
- **WHEN** an MCP server is configured and started
- **THEN** the system sends `tools/list` and registers the returned tools in the tool registry

#### Scenario: Call MCP tool
- **WHEN** the agent requests a tool that is provided by an MCP server
- **THEN** the system sends a `tools/call` request to the MCP server and returns the result to the agent

### Requirement: MCP server lifecycle management
The system SHALL start configured MCP servers on agent startup and shut them down cleanly (via process termination) on agent shutdown. The system SHALL handle MCP server crashes by emitting an error and removing the server's tools from the registry.

#### Scenario: Server crash
- **WHEN** an MCP server process exits unexpectedly
- **THEN** the system emits an error message and the server's tools become unavailable

#### Scenario: Clean shutdown
- **WHEN** the agent shuts down
- **THEN** all MCP server processes receive SIGTERM and are waited on

### Requirement: Unified tool dispatch
MCP tools and built-in tools SHALL implement the same `Tool` interface. The agent loop SHALL dispatch tool calls by name without knowing whether the tool is built-in or provided by MCP.

#### Scenario: Mixed tool calls
- **WHEN** the agent requests both a built-in tool (Read) and an MCP tool in the same turn
- **THEN** both tools execute through the same dispatch mechanism and results are returned uniformly

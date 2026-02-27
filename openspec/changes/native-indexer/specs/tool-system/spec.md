## MODIFIED Requirements

### Requirement: Tool interface
The system SHALL define a `Tool` interface with methods: `Name() string`, `Description() string`, `Schema() json.RawMessage`, and `Execute(ctx context.Context, input json.RawMessage) (ToolResult, error)`. All tools (built-in, MCP, and index) SHALL implement this interface.

#### Scenario: Tool registration
- **WHEN** a tool is registered with the registry
- **THEN** it is available for dispatch by name and its schema is included in API requests

#### Scenario: Conditional index tool registration
- **WHEN** the tool registry is initialized and a shire index database is available
- **THEN** all 13 code search tools are registered alongside the standard built-in tools

#### Scenario: No index available
- **WHEN** the tool registry is initialized and no shire index database is found
- **THEN** only the standard built-in tools are registered

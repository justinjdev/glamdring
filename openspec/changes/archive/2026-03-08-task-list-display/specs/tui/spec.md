## MODIFIED Requirements

### Requirement: Tool call display
The system SHALL display tool calls inline in the output. Each tool call SHALL show the tool name and a summary of its input. Tool results SHALL be collapsible (collapsed by default for large outputs). The `TodoWrite` tool SHALL be excluded from this behavior — its calls and results are handled by the task list display capability instead.

#### Scenario: Bash tool display
- **WHEN** the agent executes a Bash tool call with command `git status`
- **THEN** the TUI displays something like "Bash: git status" followed by the output

#### Scenario: Large tool result
- **WHEN** a tool result exceeds a configurable line threshold
- **THEN** the result is collapsed by default with an option to expand

#### Scenario: TodoWrite call is not shown as a generic tool call
- **WHEN** the agent executes a `TodoWrite` tool call
- **THEN** no generic tool call block is added to the output; a task list block is shown instead

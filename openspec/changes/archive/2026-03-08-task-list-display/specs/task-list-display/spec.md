## ADDED Requirements

### Requirement: Render TodoWrite tool calls as a task list block
The system SHALL intercept `ToolCall` messages with tool name `TodoWrite` and render them as a live task list block rather than a generic tool call block. The task list block SHALL display each task's content and status using distinct visual indicators.

#### Scenario: First TodoWrite call renders task list
- **WHEN** the agent emits a `ToolCall` message with `ToolName == "TodoWrite"` containing a todos array
- **THEN** a task list block appears in the output with one row per task, each showing its status indicator and content

#### Scenario: Pending task indicator
- **WHEN** a task has status `"pending"`
- **THEN** it is rendered with a `[ ]` prefix

#### Scenario: In-progress task indicator
- **WHEN** a task has status `"in_progress"`
- **THEN** it is rendered with a `[>]` prefix

#### Scenario: Completed task indicator
- **WHEN** a task has status `"completed"`
- **THEN** it is rendered with a `[x]` prefix

#### Scenario: Malformed todos entry is skipped
- **WHEN** a todos entry is missing required fields or has an unrecognized status
- **THEN** that entry is silently skipped and valid entries are still rendered

### Requirement: Update task list in-place across multiple TodoWrite calls
The system SHALL update the existing task list block in-place when subsequent `TodoWrite` calls arrive during the same agent turn, rather than appending a new block.

#### Scenario: Second TodoWrite call updates existing block
- **WHEN** the agent emits a second `TodoWrite` call during the same turn
- **THEN** the task list block is updated to reflect the new task statuses without a new block appearing below it

#### Scenario: New turn starts fresh task list
- **WHEN** a new agent turn begins and the agent emits a `TodoWrite` call
- **THEN** a new task list block is created rather than updating the previous turn's block

### Requirement: Suppress TodoWrite tool result block
The system SHALL NOT render a tool result block for TodoWrite tool calls. The acknowledgment response is not user-visible information.

#### Scenario: TodoWrite result produces no output block
- **WHEN** the agent emits a `ToolResult` message for a `TodoWrite` call
- **THEN** no tool result block is appended to the output

### Requirement: Finalize task list when the turn completes
The system SHALL finalize the task list block when the agent turn ends, preventing further in-place updates.

#### Scenario: Task list frozen after turn
- **WHEN** the agent emits `MessageDone`
- **THEN** the task list block is marked finalized and its rendered output is cached

#### Scenario: Task list remains visible after turn
- **WHEN** the agent turn completes and the task list block was the last block
- **THEN** the task list remains visible in the output in its final state

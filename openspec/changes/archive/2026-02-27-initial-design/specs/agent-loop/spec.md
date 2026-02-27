## ADDED Requirements

### Requirement: Execute agentic loop until completion
The system SHALL implement a loop that sends a message to the API, inspects `stop_reason`, executes any requested tools, sends tool results back, and repeats. The loop SHALL terminate when `stop_reason` is `end_turn` or `refusal`.

#### Scenario: Simple text response
- **WHEN** the API response has `stop_reason: "end_turn"` with no tool_use blocks
- **THEN** the loop emits the text content and terminates

#### Scenario: Single tool call
- **WHEN** the API response has `stop_reason: "tool_use"` with one tool_use block
- **THEN** the loop executes the tool, appends the assistant response and tool_result to the conversation, and sends the next request

#### Scenario: Multiple tool calls in one response
- **WHEN** the API response contains multiple tool_use blocks
- **THEN** the loop executes all tools, sends all tool_results in a single user message, and continues

### Requirement: Respect max turns limit
The system SHALL accept a configurable maximum number of turns. When the limit is reached, the loop SHALL terminate and emit a `MaxTurnsReached` message.

#### Scenario: Turn limit exceeded
- **WHEN** the agent has completed the configured maximum number of turns
- **THEN** the loop stops and emits a `MaxTurnsReached` message with the conversation state

### Requirement: Emit structured messages on output channel
The system SHALL communicate all events via a Go channel of typed `Message` values. Message types SHALL include: `TextDelta`, `ThinkingDelta`, `ToolCall`, `ToolResult`, `PermissionRequest`, `Error`, `MaxTurnsReached`, and `Done`.

#### Scenario: Consumer receives all events
- **WHEN** the agent loop runs a complete turn with thinking, text, and tool use
- **THEN** the output channel receives messages in order: ThinkingDelta(s), TextDelta(s), ToolCall, ToolResult, and eventually Done

### Requirement: Support cancellation via context
The system SHALL accept a `context.Context` and terminate the loop cleanly when the context is cancelled. In-flight API requests SHALL be cancelled. Running tool executions SHALL be signalled to stop.

#### Scenario: User cancels mid-turn
- **WHEN** the context is cancelled while the agent is waiting for an API response
- **THEN** the HTTP request is cancelled and the loop terminates with a cancellation message

#### Scenario: User cancels during tool execution
- **WHEN** the context is cancelled while a Bash tool is running
- **THEN** the subprocess receives SIGTERM and the loop terminates

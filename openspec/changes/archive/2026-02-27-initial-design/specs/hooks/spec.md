## ADDED Requirements

### Requirement: Execute shell hooks on agent events
The system SHALL support hooks — shell commands triggered by agent lifecycle events. Hooks SHALL be defined in `.claude/settings.json` or equivalent configuration. Each hook specifies an event, an optional matcher (regex on tool name), and a command to execute.

#### Scenario: PostToolUse hook fires
- **WHEN** a hook is configured for `PostToolUse` with matcher `Edit|Write` and the agent completes an Edit tool call
- **THEN** the hook command is executed with the tool name and input available as environment variables or stdin

#### Scenario: Hook does not match
- **WHEN** a hook is configured for `PostToolUse` with matcher `Bash` and the agent completes a Read tool call
- **THEN** the hook command is NOT executed

### Requirement: Supported hook events
The system SHALL support the following hook events: `PreToolUse`, `PostToolUse`, `SessionStart`, `SessionEnd`, `Stop`.

#### Scenario: SessionStart hook
- **WHEN** the agent session begins
- **THEN** all hooks registered for `SessionStart` are executed before the first prompt is sent

### Requirement: Hook failure handling
The system SHALL treat hook failures (non-zero exit) as warnings by default. A `PreToolUse` hook that fails SHALL block the tool execution and return an error to the agent. A `PostToolUse` hook that fails SHALL log a warning but not affect the agent loop.

#### Scenario: PreToolUse hook blocks execution
- **WHEN** a `PreToolUse` hook exits with non-zero status
- **THEN** the tool is not executed and the agent receives an error result with the hook's stderr

#### Scenario: PostToolUse hook fails
- **WHEN** a `PostToolUse` hook exits with non-zero status
- **THEN** a warning is emitted but the agent loop continues normally

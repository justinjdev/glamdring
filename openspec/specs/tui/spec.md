# tui Specification

## Purpose
TBD - created by archiving change initial-design. Update Purpose after archive.
## Requirements
### Requirement: Streaming text output with markdown rendering
The system SHALL render agent text output as styled markdown in the terminal using glamour. Text SHALL appear incrementally as `TextDelta` messages arrive from the agent. Code blocks SHALL be syntax-highlighted.

#### Scenario: Streaming text display
- **WHEN** the agent streams text deltas
- **THEN** text appears in the terminal incrementally without waiting for the full response

#### Scenario: Code block rendering
- **WHEN** the agent output contains a fenced code block with a language identifier
- **THEN** the code block is rendered with syntax highlighting

### Requirement: Multiline text input
The system SHALL provide a text input area that supports multiline editing. The input SHALL support standard readline-style keybindings. Enter SHALL submit the input; Shift+Enter or a configurable key SHALL insert a newline.

#### Scenario: Multiline input
- **WHEN** the user presses Shift+Enter
- **THEN** a newline is inserted in the input rather than submitting

#### Scenario: Submit input
- **WHEN** the user presses Enter
- **THEN** the input content is sent as the next user message and the input is cleared

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

### Requirement: Permission prompt UI
The system SHALL display permission prompts as a modal or inline prompt when the agent requests a side-effect tool. The prompt SHALL show the tool name, input details, and accept y (yes), n (no), or a (always allow) as responses.

#### Scenario: Permission prompt interaction
- **WHEN** the agent requests to run `rm -rf /tmp/test`
- **THEN** the TUI displays the command and waits for y/n/a input before proceeding

### Requirement: Status bar
The system SHALL display a persistent status bar showing: model name, token usage (input/output), estimated cost, and current turn number.

#### Scenario: Status updates after response
- **WHEN** the agent completes a turn
- **THEN** the status bar updates with the cumulative token count and cost

### Requirement: Thinking display
The system SHALL display thinking blocks in a visually distinct style (dimmed, italic, or collapsible). Thinking content SHALL stream incrementally like text output.

#### Scenario: Thinking followed by response
- **WHEN** the agent produces thinking blocks followed by text
- **THEN** the thinking is displayed in a distinct visual style, followed by the response text in normal style

### Requirement: Slash command input
The system SHALL detect when user input begins with `/` and treat it as a slash command. The system SHALL provide tab completion for available commands.

#### Scenario: Slash command execution
- **WHEN** the user types `/review auth.go` and presses Enter
- **THEN** the system expands the command template and sends the expanded prompt

### Requirement: Scrollable output viewport
The system SHALL maintain a scrollable viewport for the conversation history. The user SHALL be able to scroll up to see previous messages and tool outputs. New output SHALL auto-scroll to bottom unless the user has scrolled up.

#### Scenario: Auto-scroll
- **WHEN** new text arrives and the viewport is at the bottom
- **THEN** the viewport scrolls to show the new content

#### Scenario: Manual scroll preserved
- **WHEN** the user has scrolled up and new text arrives
- **THEN** the viewport stays at the user's scroll position and does not jump to the bottom

### Requirement: Index command
The system SHALL provide a `/index` built-in command. When invoked with no arguments or `status`, it SHALL display the current index status (package count, symbol count, file count, last build time, build duration). When invoked with `rebuild`, it SHALL shell out to `shire build` and display the result. When `shire` is not on PATH, the rebuild subcommand SHALL display an error with install instructions.

#### Scenario: Index status display
- **WHEN** the user types `/index`
- **THEN** the TUI displays the current index status including package count, symbol count, file count, and last build timestamp

#### Scenario: Manual rebuild
- **WHEN** the user types `/index rebuild` and `shire` is on PATH
- **THEN** the TUI runs `shire build --root <cwd>`, reopens the database, and displays the result summary

#### Scenario: No index available
- **WHEN** the user types `/index` but no `.shire/index.db` exists
- **THEN** the TUI displays a message indicating no index exists and suggests running `/index rebuild` or `shire build`

#### Scenario: Shire not installed
- **WHEN** the user types `/index rebuild` but `shire` is not on PATH
- **THEN** the TUI displays an error message with shire install instructions


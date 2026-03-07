## MODIFIED Requirements

### Requirement: Execute shell hooks on agent events
The system SHALL support hooks -- shell commands triggered by agent lifecycle events. Hooks SHALL be defined in `.glamdring/config.json` or `.claude/settings.json` (with `.glamdring/config.json` checked first using centralized path resolution). Each hook specifies an event, an optional matcher (regex on tool name), and a command to execute.

#### Scenario: PostToolUse hook fires
- **WHEN** a hook is configured for `PostToolUse` with matcher `Edit|Write` and the agent completes an Edit tool call
- **THEN** the hook command is executed with the tool name and input available as environment variables or stdin

#### Scenario: Hook does not match
- **WHEN** a hook is configured for `PostToolUse` with matcher `Bash` and the agent completes a Read tool call
- **THEN** the hook command is NOT executed

#### Scenario: Hooks loaded from .glamdring/config.json
- **WHEN** `.glamdring/config.json` contains a `hooks` key and `.claude/settings.json` also contains hooks
- **THEN** hooks SHALL be loaded from `.glamdring/config.json` only

#### Scenario: Hooks fallback to .claude/settings.json
- **WHEN** no `.glamdring/config.json` exists and `.claude/settings.json` contains hooks
- **THEN** hooks SHALL be loaded from `.claude/settings.json`

#### Scenario: Hooks from multiple directory levels
- **WHEN** hooks are defined in both user-level (`~/.config/glamdring/config.json`) and project-level (`.glamdring/config.json`)
- **THEN** hooks from both levels SHALL be combined

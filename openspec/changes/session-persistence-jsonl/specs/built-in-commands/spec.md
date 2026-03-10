## ADDED Requirements

### Requirement: /session command with subcommands
The system SHALL implement a `/session` built-in command with three subcommands: `list`, `resume`, and `delete`. An unknown subcommand SHALL display usage. Running `/session` with no subcommand SHALL behave identically to `/session list`.

#### Scenario: /session list
- **WHEN** the user types `/session list` or `/session`
- **THEN** the system displays all stored sessions in reverse-chronological order, showing ID (truncated to 8 chars), title, date, and message count

#### Scenario: /session resume
- **WHEN** the user types `/session resume <id>`
- **THEN** the system loads the session's messages and replaces the current conversation history, displaying a confirmation message

#### Scenario: /session delete
- **WHEN** the user types `/session delete <id>`
- **THEN** the system deletes the session file and updates the index, displaying a confirmation message

#### Scenario: /session repair
- **WHEN** the user types `/session repair`
- **THEN** the system scans all JSONL files and rebuilds the index, displaying the number of sessions found

#### Scenario: /session with no subcommand
- **WHEN** the user types `/session`
- **THEN** the system behaves identically to `/session list`

#### Scenario: Unknown subcommand
- **WHEN** the user types `/session <unknown>`
- **THEN** the system displays usage: `Usage: /session [list|resume <id>|delete <id>|repair]`

#### Scenario: Persistence disabled
- **WHEN** the user types any `/session` subcommand and `persistence.enabled` is false
- **THEN** the system displays a message indicating session persistence is disabled

### Requirement: /clear starts a new session
When persistence is enabled, `/clear` SHALL close the current session (finalizing its index entry) before resetting the conversation.

#### Scenario: /clear with persistence enabled
- **WHEN** the user types `/clear` and persistence is enabled
- **THEN** the current session is finalized in the index and a new session UUID is generated before the conversation is reset

#### Scenario: /clear with persistence disabled
- **WHEN** the user types `/clear` and persistence is disabled
- **THEN** behavior is unchanged from the existing /clear requirement

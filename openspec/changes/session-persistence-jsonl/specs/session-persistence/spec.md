## ADDED Requirements

### Requirement: Persist session messages to JSONL on each turn
The system SHALL append new messages to a per-session JSONL file after each agent turn completes. Each line SHALL be a valid JSON encoding of one `api.RequestMessage`. The file SHALL be located at `<sessions-dir>/<uuid>.jsonl`.

#### Scenario: Messages written after assistant response
- **WHEN** an agent turn completes and the assistant response is appended to the conversation
- **THEN** all new messages since the last save are appended to the session's JSONL file, one per line

#### Scenario: Persistence disabled
- **WHEN** `persistence.enabled` is false in config
- **THEN** no JSONL files are created and session behavior is unchanged

#### Scenario: Sessions directory does not exist
- **WHEN** the sessions directory does not exist on first write
- **THEN** the system creates the directory before writing the file

### Requirement: Assign a UUID to each session
The system SHALL generate a UUID (v4) for each new session at startup. The UUID SHALL be used as the JSONL filename and as the session identifier in the index.

#### Scenario: New session gets unique ID
- **WHEN** glamdring starts and creates a new session
- **THEN** a unique UUID is assigned and `<uuid>.jsonl` is created on first write

### Requirement: Maintain a session index file
The system SHALL maintain `<sessions-dir>/index.json` containing an array of session metadata entries. Each entry SHALL include: `id`, `title`, `created_at`, `updated_at`, `message_count`. The index SHALL be rewritten atomically (write-then-rename) on session close.

#### Scenario: Index updated on session close
- **WHEN** a session ends (process exit or `/clear`)
- **THEN** the index file is updated with the session's final metadata

#### Scenario: Index written atomically
- **WHEN** the index is rewritten
- **THEN** the system writes to a `.tmp` file and renames it to `index.json`, preventing partial reads

#### Scenario: Index rebuild when corrupted
- **WHEN** the user runs `/session repair`
- **THEN** the system scans all `*.jsonl` files in the sessions directory and rebuilds `index.json` from their contents

### Requirement: Restore a previous session
The system SHALL support loading a prior session's messages from its JSONL file and setting them as the active conversation history.

#### Scenario: Restore session by ID
- **WHEN** the user runs `/session resume <id>`
- **THEN** the system reads the JSONL file for that session ID, deserializes all messages, and replaces the current conversation history

#### Scenario: Restore nonexistent session
- **WHEN** the user runs `/session resume <id>` and no file exists for that ID
- **THEN** the system displays an error and leaves the current conversation unchanged

### Requirement: Startup restore prompt
The system SHALL offer to restore the most recent session at startup when persistence is enabled and a prior session exists.

#### Scenario: Prior session exists
- **WHEN** glamdring starts and `index.json` contains at least one prior session
- **THEN** the TUI displays a one-line prompt asking the user to resume the most recent session (default: N)

#### Scenario: User accepts restore
- **WHEN** the user responds Y to the restore prompt
- **THEN** the prior session's messages are loaded as the active conversation

#### Scenario: User declines restore
- **WHEN** the user responds N (or presses Enter) at the restore prompt
- **THEN** a fresh session is started with no prior messages

#### Scenario: No prior session
- **WHEN** `index.json` is empty or does not exist
- **THEN** no restore prompt is shown

### Requirement: List sessions
The system SHALL display all stored sessions when the user runs `/session list`, showing ID, title, creation date, and message count in reverse-chronological order.

#### Scenario: Sessions exist
- **WHEN** the user runs `/session list`
- **THEN** the system reads `index.json` and displays each session in reverse-chronological order

#### Scenario: No sessions
- **WHEN** the user runs `/session list` and no sessions are stored
- **THEN** the system displays a message indicating no sessions found

### Requirement: Delete a session
The system SHALL delete a session's JSONL file and remove it from the index when the user runs `/session delete <id>`.

#### Scenario: Delete existing session
- **WHEN** the user runs `/session delete <id>` and the session exists
- **THEN** the JSONL file is deleted and the index is updated atomically

#### Scenario: Delete nonexistent session
- **WHEN** the user runs `/session delete <id>` and no file exists for that ID
- **THEN** the system displays an error and leaves the index unchanged

### Requirement: Auto-derive session title
The system SHALL derive the session title from the first 60 characters of the first user message. If no user message has been sent, the title SHALL be "New Session".

#### Scenario: Title from first user message
- **WHEN** the first user message is sent in a session
- **THEN** the session title is set to the first 60 characters of that message content

#### Scenario: No user message yet
- **WHEN** a session is closed before any user message is sent
- **THEN** the session title is "New Session"

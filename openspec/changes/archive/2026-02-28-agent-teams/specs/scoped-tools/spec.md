## ADDED Requirements

### Requirement: Path-scoped write tools
When a team agent claims a task with scope path patterns, the Edit and Write tools SHALL be wrapped to validate that the target file path matches the allowed patterns before execution. If the path is outside scope, the tool SHALL return an error without modifying the filesystem.

#### Scenario: Write within scope succeeds
- **WHEN** an agent with scope ["pkg/auth/**"] calls Write with path "pkg/auth/handler.go"
- **THEN** the write executes normally

#### Scenario: Write outside scope fails
- **WHEN** an agent with scope ["pkg/auth/**"] calls Write with path "pkg/api/client.go"
- **THEN** the tool returns an error: "Path pkg/api/client.go is outside task scope [pkg/auth/**]. Request scope change via SendMessage to team lead."

#### Scenario: No scope restrictions
- **WHEN** an agent claims a task with no scope metadata and calls Write
- **THEN** the write executes normally with no path checking

### Requirement: Advisory command scoping for Bash
Bash command scoping is advisory, not a security boundary. Shell command pattern matching is fundamentally broken against subshells, backtick execution, pipes, eval, xargs, process substitution, and heredocs. The primary enforcement for Bash is schema-level exclusion: Bash SHALL NOT appear in the API tools array during research and plan phases (agents cannot call what they cannot see). In implement and verify phases, Bash SHALL be available with optional allow-list patterns as behavioral hints. Deny patterns SHALL NOT be used (they provide false confidence against trivial bypasses). If no allow patterns are defined, all commands are permitted.

#### Scenario: Allowed command executes
- **WHEN** an agent with allow commands ["go test*", "go build*"] calls Bash with "go test ./pkg/auth/..."
- **THEN** the command executes normally

#### Scenario: Command not in allow list
- **WHEN** an agent with allow commands ["go test*"] calls Bash with "npm install"
- **THEN** the tool returns an advisory error: "Command 'npm install' does not match allowed patterns [go test*]. Request scope change via SendMessage to team lead."

#### Scenario: No allow patterns means unrestricted
- **WHEN** an agent has no allow command patterns defined and calls Bash with any command
- **THEN** the command executes normally

#### Scenario: Bash excluded by schema in research phase
- **WHEN** an agent in "research" phase attempts to call Bash
- **THEN** Bash is not in the API tools array -- the model cannot generate the call (this is the real enforcement)

### Requirement: File locking
When a team agent successfully writes or edits a file, the system SHALL acquire a file lock for that path associated with the agent. Subsequent write/edit attempts by other agents on the same path SHALL fail with an error identifying the lock holder.

#### Scenario: First write acquires lock
- **WHEN** agent "auth-impl" successfully writes to "pkg/auth/handler.go"
- **THEN** the file is locked to "auth-impl"

#### Scenario: Second agent blocked by lock
- **WHEN** agent "api-impl" attempts to edit "pkg/auth/handler.go" which is locked by "auth-impl"
- **THEN** the tool returns an error: "File locked by agent 'auth-impl'. Coordinate via SendMessage."

#### Scenario: Same agent can write to locked file
- **WHEN** agent "auth-impl" writes again to "pkg/auth/handler.go" which it already locked
- **THEN** the write executes normally (agent holds the lock)

#### Scenario: Locks released on task completion
- **WHEN** agent "auth-impl" marks its task as completed
- **THEN** all file locks held by "auth-impl" are released

### Requirement: Mandatory check-in enforcement
The system SHALL track the number of non-read tool calls each team agent makes since its last SendMessage or TaskUpdate. When the count exceeds a configurable threshold (default 15), the next non-read tool call SHALL return an error requiring the agent to report progress before continuing.

#### Scenario: Check-in required after threshold
- **WHEN** agent "auth-impl" has made 15 Edit/Write/Bash calls without a SendMessage or TaskUpdate
- **THEN** the next Edit call returns an error: "Progress check-in required. Send a status update via SendMessage or TaskUpdate before continuing. (15 tool calls since last check-in)"

#### Scenario: SendMessage resets counter
- **WHEN** agent "auth-impl" calls SendMessage to report progress
- **THEN** the check-in counter resets to 0 and subsequent tool calls proceed normally

#### Scenario: TaskUpdate resets counter
- **WHEN** agent "auth-impl" calls TaskUpdate to update task status
- **THEN** the check-in counter resets to 0

#### Scenario: Read-only tools do not increment counter
- **WHEN** agent "auth-impl" calls Read, Glob, or Grep
- **THEN** the check-in counter is not incremented

### Requirement: Decorator composition order
Scoped tool wrappers SHALL be composed in a fixed order: CheckinGate (outermost) -> FileLock -> ScopedTool -> BaseTool (innermost). This ensures check-in is validated first, then file locking, then scope, then the actual operation.

#### Scenario: All checks pass
- **WHEN** an agent calls Edit and the check-in counter is under threshold, the file is unlocked or self-locked, and the path is in scope
- **THEN** all decorators pass and the base Edit tool executes

#### Scenario: Check-in fails before lock check
- **WHEN** an agent calls Edit but the check-in counter exceeds the threshold
- **THEN** the check-in error is returned without attempting to acquire a file lock or check scope

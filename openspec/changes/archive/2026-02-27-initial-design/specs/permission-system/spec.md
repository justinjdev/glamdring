## ADDED Requirements

### Requirement: Three-tier permission model
The system SHALL classify tools into three permission tiers: always-allow (read-only tools), prompt-user (side-effect tools), and always-block (configurable deny list). The tier assignment SHALL be configurable.

#### Scenario: Read-only tool executes without prompt
- **WHEN** the agent calls Read, Glob, or Grep
- **THEN** the tool executes immediately without user confirmation

#### Scenario: Side-effect tool prompts user
- **WHEN** the agent calls Write, Edit, or Bash
- **THEN** the system emits a `PermissionRequest` message and waits for approval before executing

#### Scenario: Blocked tool is rejected
- **WHEN** the agent calls a tool on the deny list
- **THEN** the tool is not executed and the agent receives an error result

### Requirement: Permission responses
The system SHALL support three user responses to permission prompts: approve (execute this once), always-approve (approve this tool for the rest of the session), and deny (reject and return error to agent).

#### Scenario: Always-approve
- **WHEN** the user responds with always-approve for the Edit tool
- **THEN** subsequent Edit calls in the same session execute without prompting

#### Scenario: Deny
- **WHEN** the user denies a Bash tool call
- **THEN** the agent receives a tool_result with is_error: true explaining the user denied the action

### Requirement: Permission request includes context
Each `PermissionRequest` message SHALL include the tool name, the full input parameters, and a human-readable summary of what the tool will do (e.g., "Write to /path/to/file.go" or "Run: git status").

#### Scenario: Bash permission shows command
- **WHEN** a Bash tool call requires permission
- **THEN** the permission request displays the full command string to the user

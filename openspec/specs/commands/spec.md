# commands Specification

## Purpose
TBD - created by archiving change initial-design. Update Purpose after archive.
## Requirements
### Requirement: Discover slash commands from directories
The system SHALL scan `.claude/commands/` in both project-level and user-level locations for markdown files. Each `.md` file becomes a slash command named after its path (e.g., `.claude/commands/review.md` → `/review`). Subdirectories create namespaced commands (e.g., `.claude/commands/opsx/new.md` → `/opsx new`).

#### Scenario: Project-level command
- **WHEN** `.claude/commands/review.md` exists in the project
- **THEN** `/review` is available as a slash command

#### Scenario: Nested command
- **WHEN** `.claude/commands/opsx/new.md` exists
- **THEN** `/opsx new` is available as a slash command

### Requirement: Expand commands as prompt templates
When a slash command is invoked, the system SHALL read the markdown file contents and inject them as the user prompt. The file contents MAY contain template variables (e.g., `$ARGUMENTS`) that are replaced with any arguments provided after the command name.

#### Scenario: Command with arguments
- **WHEN** the user types `/review auth.go`
- **THEN** the system reads `review.md`, replaces `$ARGUMENTS` with `auth.go`, and sends the expanded content as the prompt

#### Scenario: Command with no arguments
- **WHEN** the user types `/review` with no arguments
- **THEN** the system reads `review.md`, replaces `$ARGUMENTS` with empty string, and sends the expanded content

### Requirement: List available commands
The system SHALL provide a way to list all available slash commands with their source (project or user level).

#### Scenario: List commands
- **WHEN** the user requests to see available commands
- **THEN** the system displays all discovered commands with their names and source paths


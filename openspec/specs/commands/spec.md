# commands Specification

## Purpose
Slash commands discovered from markdown files, providing user-defined prompt templates invoked via `/command` syntax.

## Requirements
### Requirement: Discover slash commands from directories
The system SHALL scan command directories in both project-level and user-level locations for markdown files. At each level, the system SHALL check `.glamdring/commands/` first and `.claude/commands/` as fallback (using centralized directory resolution). Each `.md` file becomes a slash command named after its path. Subdirectories create namespaced commands.

#### Scenario: Project-level command in .glamdring/
- **WHEN** `.glamdring/commands/review.md` exists in the project
- **THEN** `/review` is available as a slash command

#### Scenario: Project-level command fallback to .claude/
- **WHEN** `.glamdring/commands/` does not exist and `.claude/commands/review.md` exists
- **THEN** `/review` is available as a slash command

#### Scenario: Nested command
- **WHEN** `.glamdring/commands/opsx/new.md` exists
- **THEN** `/opsx new` is available as a slash command

#### Scenario: .glamdring/ commands take precedence
- **WHEN** both `.glamdring/commands/review.md` and `.claude/commands/review.md` exist
- **THEN** the command content SHALL be loaded from `.glamdring/commands/review.md`

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

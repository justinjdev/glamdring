## MODIFIED Requirements

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

## ADDED Requirements

### Requirement: Discover and load CLAUDE.md files
The system SHALL search for CLAUDE.md files at two levels: project-level (`.claude/CLAUDE.md` walking up from CWD to filesystem root) and user-level (`~/.claude/CLAUDE.md`). Both SHALL be loaded and merged into the system prompt, with project-level taking precedence.

#### Scenario: Project and user CLAUDE.md both exist
- **WHEN** both `~/.claude/CLAUDE.md` and `.claude/CLAUDE.md` exist
- **THEN** both are included in the system prompt, with the project-level content taking precedence

#### Scenario: No CLAUDE.md found
- **WHEN** no CLAUDE.md files exist at any level
- **THEN** the system proceeds with the default system prompt without error

#### Scenario: Nested project detection
- **WHEN** CWD is `/Users/me/projects/app/src/utils`
- **THEN** the system walks up and finds `.claude/CLAUDE.md` at `/Users/me/projects/app/.claude/CLAUDE.md`

### Requirement: Assemble system prompt
The system SHALL construct the system prompt by combining: base agent instructions, tool descriptions (generated from registered tool schemas), and CLAUDE.md content. The assembled prompt SHALL be sent as the `system` parameter in API requests.

#### Scenario: System prompt includes tool descriptions
- **WHEN** tools Read, Write, Edit, Bash, Glob, and Grep are registered
- **THEN** the system prompt includes a description of each tool and its parameters

### Requirement: Working directory resolution
The system SHALL accept an optional `--cwd` flag. If not provided, the system SHALL use the current working directory. All file operations SHALL be relative to the resolved working directory.

#### Scenario: Explicit CWD
- **WHEN** the user runs `glamdring --cwd /path/to/project`
- **THEN** all file tool operations are rooted at `/path/to/project` and CLAUDE.md is searched from there

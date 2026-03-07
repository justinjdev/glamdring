## MODIFIED Requirements

### Requirement: Discover and load CLAUDE.md files
The system SHALL search for instruction files at two levels: project-level (walking up from CWD to filesystem root) and user-level. At each directory level, the system SHALL check for both glamdring-namespaced and claude-namespaced instruction files, concatenating all found contents. The glamdring-namespaced files SHALL be checked first at each level.

Project-level check order at each directory:
1. `GLAMDRING.md` (bare)
2. `.glamdring/GLAMDRING.md`
3. `.glamdring/GLAMDRING.local.md`
4. `CLAUDE.md` (bare, fallback)
5. `.claude/CLAUDE.md` (fallback)
6. `.claude/CLAUDE.local.md` (fallback)

User-level check order:
1. `~/.config/glamdring/GLAMDRING.md`
2. `~/.claude/CLAUDE.md` (fallback)

All found files are concatenated (innermost directory first).

#### Scenario: Project and user instruction files both exist
- **WHEN** both `~/.config/glamdring/GLAMDRING.md` and `.glamdring/GLAMDRING.md` exist
- **THEN** both are included in the system prompt, with the project-level content taking precedence

#### Scenario: No instruction files found
- **WHEN** no GLAMDRING.md or CLAUDE.md files exist at any level
- **THEN** the system proceeds with the default system prompt without error

#### Scenario: Nested project detection
- **WHEN** CWD is `/Users/me/projects/app/src/utils`
- **THEN** the system walks up and finds instruction files at each directory level

#### Scenario: Mixed namespaces at same level
- **WHEN** a directory contains both `GLAMDRING.md` and `CLAUDE.md`
- **THEN** both files are included in the system prompt (glamdring content first, then claude content)

#### Scenario: Fallback to CLAUDE.md only
- **WHEN** no GLAMDRING.md files exist but `CLAUDE.md` exists at the project root
- **THEN** the system loads `CLAUDE.md` content into the system prompt

### Requirement: Working directory resolution
The system SHALL accept an optional `--cwd` flag. If not provided, the system SHALL use the current working directory. All file operations SHALL be relative to the resolved working directory.

#### Scenario: Explicit CWD
- **WHEN** the user runs `glamdring --cwd /path/to/project`
- **THEN** all file tool operations are rooted at `/path/to/project` and instruction files are searched from there

## MODIFIED Requirements

### Requirement: Assemble system prompt
The system SHALL construct the system prompt by combining: base agent instructions, tool descriptions (generated from registered tool schemas), and instruction file content (from GLAMDRING.md and/or CLAUDE.md). The assembled prompt SHALL be sent as the `system` parameter in API requests.

#### Scenario: System prompt includes tool descriptions
- **WHEN** tools Read, Write, Edit, Bash, Glob, and Grep are registered
- **THEN** the system prompt includes a description of each tool and its parameters

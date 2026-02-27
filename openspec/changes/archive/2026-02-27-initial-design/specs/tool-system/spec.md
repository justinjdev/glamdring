## ADDED Requirements

### Requirement: Tool interface
The system SHALL define a `Tool` interface with methods: `Name() string`, `Description() string`, `Schema() json.RawMessage`, and `Execute(ctx context.Context, input json.RawMessage) (ToolResult, error)`. All tools (built-in and MCP) SHALL implement this interface.

#### Scenario: Tool registration
- **WHEN** a tool is registered with the registry
- **THEN** it is available for dispatch by name and its schema is included in API requests

### Requirement: Read tool
The system SHALL implement a Read tool that reads file contents given an absolute path. It SHALL support optional `offset` and `limit` parameters for reading portions of large files. It SHALL return file contents with line numbers.

#### Scenario: Read entire file
- **WHEN** Read is called with a valid file path and no offset/limit
- **THEN** the tool returns the file contents with line numbers prefixed

#### Scenario: Read nonexistent file
- **WHEN** Read is called with a path that does not exist
- **THEN** the tool returns an error result with a descriptive message

### Requirement: Write tool
The system SHALL implement a Write tool that creates or overwrites a file at the given absolute path with the provided content.

#### Scenario: Create new file
- **WHEN** Write is called with a path that does not exist
- **THEN** the file is created with the specified content, including any necessary parent directories

#### Scenario: Overwrite existing file
- **WHEN** Write is called with a path to an existing file
- **THEN** the file contents are replaced with the specified content

### Requirement: Edit tool
The system SHALL implement an Edit tool that performs exact string replacement in a file. It SHALL accept `file_path`, `old_string`, `new_string`, and optional `replace_all` parameters. The edit SHALL fail if `old_string` is not found or is not unique (unless `replace_all` is true).

#### Scenario: Unique string replacement
- **WHEN** Edit is called with an `old_string` that appears exactly once in the file
- **THEN** the occurrence is replaced with `new_string`

#### Scenario: Ambiguous match
- **WHEN** Edit is called with an `old_string` that appears multiple times and `replace_all` is false
- **THEN** the tool returns an error indicating the match is not unique

#### Scenario: Replace all
- **WHEN** Edit is called with `replace_all: true`
- **THEN** all occurrences of `old_string` are replaced with `new_string`

### Requirement: Bash tool
The system SHALL implement a Bash tool that executes shell commands and returns stdout, stderr, and exit code. It SHALL support an optional timeout parameter. Commands SHALL run in the agent's working directory.

#### Scenario: Successful command
- **WHEN** Bash is called with a command that exits 0
- **THEN** the tool returns stdout and stderr with exit code 0

#### Scenario: Command timeout
- **WHEN** Bash is called with a command that exceeds the timeout
- **THEN** the process is killed and the tool returns a timeout error

#### Scenario: Command failure
- **WHEN** Bash is called with a command that exits non-zero
- **THEN** the tool returns stdout, stderr, and the non-zero exit code

### Requirement: Glob tool
The system SHALL implement a Glob tool that finds files matching a glob pattern (e.g., `**/*.go`, `src/**/*.ts`). It SHALL return matching file paths sorted by modification time.

#### Scenario: Pattern match
- **WHEN** Glob is called with a pattern that matches files
- **THEN** the tool returns the matching file paths

#### Scenario: No matches
- **WHEN** Glob is called with a pattern that matches no files
- **THEN** the tool returns an empty result

### Requirement: Grep tool
The system SHALL implement a Grep tool that searches file contents using regular expressions. It SHALL support parameters for: pattern, path, glob filter, output mode (content, files_with_matches, count), context lines, and case-insensitive search.

#### Scenario: Content search
- **WHEN** Grep is called with a pattern and output_mode "content"
- **THEN** the tool returns matching lines with line numbers and optional context

#### Scenario: File list search
- **WHEN** Grep is called with output_mode "files_with_matches"
- **THEN** the tool returns only the paths of files containing matches

## ADDED Requirements

### Requirement: Index command
The system SHALL provide a `/index` built-in command. When invoked with no arguments or `status`, it SHALL display the current index status (package count, symbol count, file count, last build time, build duration). When invoked with `rebuild`, it SHALL shell out to `shire build` and display the result. When `shire` is not on PATH, the rebuild subcommand SHALL display an error with install instructions.

#### Scenario: Index status display
- **WHEN** the user types `/index`
- **THEN** the TUI displays the current index status including package count, symbol count, file count, and last build timestamp

#### Scenario: Manual rebuild
- **WHEN** the user types `/index rebuild` and `shire` is on PATH
- **THEN** the TUI runs `shire build --root <cwd>`, reopens the database, and displays the result summary

#### Scenario: No index available
- **WHEN** the user types `/index` but no `.shire/index.db` exists
- **THEN** the TUI displays a message indicating no index exists and suggests running `/index rebuild` or `shire build`

#### Scenario: Shire not installed
- **WHEN** the user types `/index rebuild` but `shire` is not on PATH
- **THEN** the TUI displays an error message with shire install instructions

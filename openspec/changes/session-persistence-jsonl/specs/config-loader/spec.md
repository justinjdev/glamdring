## ADDED Requirements

### Requirement: Load persistence configuration block
The system SHALL load a `persistence` block from the settings file with fields: `enabled` (bool, default true) and `dir` (string, optional). When `dir` is not set, the system SHALL default to `~/.glamdring/sessions/`. The resolved directory SHALL be passed to the session store at startup.

#### Scenario: Persistence block present
- **WHEN** the settings file contains a `persistence` block with `enabled: true` and a custom `dir`
- **THEN** the session store is initialized with the specified directory

#### Scenario: Persistence block absent
- **WHEN** no `persistence` block is present in settings
- **THEN** persistence defaults to enabled with `~/.glamdring/sessions/` as the sessions directory

#### Scenario: Persistence explicitly disabled
- **WHEN** the settings file contains `persistence.enabled: false`
- **THEN** no session store is initialized and no JSONL files are created

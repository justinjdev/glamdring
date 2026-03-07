## ADDED Requirements

### Requirement: Primary and fallback config directory resolution
The system SHALL check `.glamdring/` as the primary config directory and `.claude/` as the fallback at both project and user levels. When resolving a config file or directory, the system SHALL return the `.glamdring/` path if it exists, otherwise the `.claude/` path if it exists, otherwise empty string.

#### Scenario: Only .glamdring/ exists
- **WHEN** `.glamdring/config.json` exists and `.claude/settings.json` does not
- **THEN** the system SHALL load config from `.glamdring/config.json`

#### Scenario: Only .claude/ exists (fallback)
- **WHEN** `.glamdring/config.json` does not exist and `.claude/settings.json` exists
- **THEN** the system SHALL load config from `.claude/settings.json`

#### Scenario: Both exist (.glamdring/ wins)
- **WHEN** both `.glamdring/config.json` and `.claude/settings.json` exist
- **THEN** the system SHALL load config from `.glamdring/config.json` only

#### Scenario: Neither exists
- **WHEN** neither `.glamdring/config.json` nor `.claude/settings.json` exists
- **THEN** the system SHALL proceed with defaults

### Requirement: User-level config directory
The system SHALL use `~/.config/glamdring/` as the primary user-level config directory, with `~/.claude/` as the fallback. The same resolution priority applies: `~/.config/glamdring/` is checked first.

#### Scenario: User config in XDG location
- **WHEN** `~/.config/glamdring/config.json` exists
- **THEN** the system SHALL load user settings from that path

#### Scenario: User config fallback to .claude
- **WHEN** `~/.config/glamdring/config.json` does not exist and `~/.claude/settings.json` exists
- **THEN** the system SHALL load user settings from `~/.claude/settings.json`

### Requirement: Project root detection
The system SHALL detect the project root by walking up from cwd looking for `.glamdring/` first, then `.claude/`. The first directory found (closest to cwd) determines the project root, regardless of which namespace it belongs to.

#### Scenario: .glamdring/ at project root
- **WHEN** `/projects/app/.glamdring/` exists
- **THEN** `FindProjectRoot("/projects/app/src/utils")` SHALL return `/projects/app`

#### Scenario: .claude/ fallback at project root
- **WHEN** `/projects/app/.claude/` exists and no `.glamdring/` exists in any ancestor
- **THEN** `FindProjectRoot("/projects/app/src/utils")` SHALL return `/projects/app`

#### Scenario: .glamdring/ closer than .claude/
- **WHEN** `/projects/app/.glamdring/` exists and `/projects/.claude/` exists
- **THEN** `FindProjectRoot("/projects/app/src")` SHALL return `/projects/app`

### Requirement: Directory-level config file resolution
The `Resolve` function SHALL accept a base directory and a relative path, then check `.glamdring/<rel>` followed by `.claude/<rel>`, returning the first existing path.

#### Scenario: Resolve permissions file
- **WHEN** `Resolve("/projects/app", "permissions.json")` is called and `.glamdring/permissions.json` exists
- **THEN** it SHALL return `/projects/app/.glamdring/permissions.json`

#### Scenario: Resolve with fallback
- **WHEN** `Resolve("/projects/app", "permissions.json")` is called and only `.claude/permissions.json` exists
- **THEN** it SHALL return `/projects/app/.claude/permissions.json`

### Requirement: Settings file naming
The primary settings file SHALL be named `config.json` (under `.glamdring/`). The fallback SHALL read `settings.json` (under `.claude/`). The file format and schema SHALL be identical.

#### Scenario: config.json loaded
- **WHEN** `.glamdring/config.json` contains `{"model": "claude-sonnet-4-6"}`
- **THEN** the system SHALL use `claude-sonnet-4-6` as the model

#### Scenario: settings.json fallback
- **WHEN** `.claude/settings.json` contains `{"model": "claude-sonnet-4-6"}` and no `.glamdring/config.json` exists
- **THEN** the system SHALL use `claude-sonnet-4-6` as the model

## MODIFIED Requirements

### Requirement: Team scope rules in permission evaluation
The permission system SHALL evaluate team scope rules (path restrictions, command restrictions) as an additional layer in the permission check. Team scope rules SHALL be evaluated after hook and config layers but before the session-level interactive prompt. A scope violation SHALL result in an immediate denial without prompting the user.

The permission config file SHALL be loaded from `.glamdring/permissions.json` with fallback to `.claude/permissions.json` using the centralized path resolution.

#### Scenario: Scope denial bypasses interactive prompt
- **WHEN** a team agent calls Edit on a path outside its task scope
- **THEN** the tool is denied immediately with a scope violation error, without emitting a PermissionRequest to the user

#### Scenario: Scope-allowed tool still checks other layers
- **WHEN** a team agent calls Bash with a command within scope but the command matches a config-level deny rule
- **THEN** the config-level deny takes precedence and the tool is rejected

#### Scenario: Permissions loaded from .glamdring/
- **WHEN** `.glamdring/permissions.json` exists
- **THEN** permissions SHALL be loaded from `.glamdring/permissions.json`

#### Scenario: Permissions fallback to .claude/
- **WHEN** `.glamdring/permissions.json` does not exist and `.claude/permissions.json` exists
- **THEN** permissions SHALL be loaded from `.claude/permissions.json`

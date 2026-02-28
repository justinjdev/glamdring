## ADDED Requirements

### Requirement: Team scope rules in permission evaluation
The permission system SHALL evaluate team scope rules (path restrictions, command restrictions) as an additional layer in the permission check. Team scope rules SHALL be evaluated after hook and config layers but before the session-level interactive prompt. A scope violation SHALL result in an immediate denial without prompting the user.

#### Scenario: Scope denial bypasses interactive prompt
- **WHEN** a team agent calls Edit on a path outside its task scope
- **THEN** the tool is denied immediately with a scope violation error, without emitting a PermissionRequest to the user

#### Scenario: Scope-allowed tool still checks other layers
- **WHEN** a team agent calls Bash with a command within scope but the command matches a config-level deny rule
- **THEN** the config-level deny takes precedence and the tool is rejected

### Requirement: Team agents inherit parent permission model
Team agents SHALL operate under the same three-tier permission model as non-team agents. The team lead's session-level approvals (sessionAllow, yolo mode) SHALL propagate to spawned team agents. Team scope rules add restrictions on top of the base permission model -- they never grant additional permissions.

#### Scenario: Yolo mode with scope
- **WHEN** the lead session is in yolo mode and a team agent calls Write on a path within scope
- **THEN** the write executes without permission prompt (yolo) and without scope error (in scope)

#### Scenario: Yolo mode does not bypass scope
- **WHEN** the lead session is in yolo mode and a team agent calls Write on a path outside scope
- **THEN** the write is rejected with a scope violation error -- yolo bypasses interactive prompts but not structural scope enforcement

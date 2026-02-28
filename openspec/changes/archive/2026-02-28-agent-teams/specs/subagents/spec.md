## MODIFIED Requirements

### Requirement: Spawn subagent tasks
The system SHALL support spawning subagents -- independent agent loops that run concurrently as goroutines. Each subagent gets its own conversation context, system prompt, and tool set. The parent agent communicates with subagents via the Task tool. The Task tool SHALL accept optional `team_name` and `name` parameters. When `team_name` is provided, the spawned agent joins the specified team and receives team-aware tools (phase registry, scoped wrappers, messaging). When `team_name` is absent, the existing isolated subagent behavior is preserved.

#### Scenario: Spawn and receive result (non-team)
- **WHEN** the parent agent calls the Task tool with a prompt and subagent type but no team_name
- **THEN** a new agent loop starts in a goroutine, executes the task, and returns the result to the parent (existing behavior unchanged)

#### Scenario: Spawn team agent
- **WHEN** the parent agent calls the Task tool with prompt, team_name "backend-refactor", and name "auth-impl"
- **THEN** a new agent loop starts, the agent is registered in the team, receives a PhaseRegistry filtered to the initial phase, and can send/receive messages via its team mailbox

#### Scenario: Team agent without name
- **WHEN** the Task tool is called with team_name but no name
- **THEN** the tool returns an error indicating that team agents require a name for message routing

### Requirement: Subagent tool restrictions
Subagents SHALL accept a tool whitelist. If specified, the subagent can only use the listed tools. If not specified, the subagent inherits the parent's tool set. For team agents, the tool set is further filtered by the PhaseRegistry based on the agent's current workflow phase.

#### Scenario: Read-only subagent (non-team)
- **WHEN** a subagent is spawned with tools restricted to [Read, Glob, Grep]
- **THEN** the subagent cannot call Write, Edit, or Bash

#### Scenario: Team agent tool set is phase-filtered
- **WHEN** a team agent is spawned with no explicit tool restrictions
- **THEN** the agent's available tools are determined by its current workflow phase, not the full parent tool set

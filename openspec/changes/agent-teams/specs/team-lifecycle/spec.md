## ADDED Requirements

### Requirement: Experimental feature gate
All team functionality SHALL be gated behind an experimental flag that defaults to disabled. When disabled, team tools (TeamCreate, TeamDelete, TaskCreate, TaskList, TaskGet, TaskUpdate, SendMessage, AdvancePhase) SHALL NOT be registered in the tool registry, and the Task tool SHALL ignore `team_name` and `name` parameters. The flag SHALL be activatable via CLI (`--experimental-teams`) or config (`"experimental": {"teams": true}` in settings.json). CLI flag SHALL override config.

#### Scenario: Teams disabled by default
- **WHEN** glamdring starts with no experimental flag and no config setting
- **THEN** team tools are not registered and the Task tool does not accept team_name or name parameters

#### Scenario: Enable via CLI flag
- **WHEN** glamdring starts with `--experimental-teams`
- **THEN** team tools are registered and the Task tool accepts team_name and name parameters

#### Scenario: Enable via config
- **WHEN** settings.json contains `"experimental": {"teams": true}` and no CLI flag is provided
- **THEN** team tools are registered

#### Scenario: CLI flag overrides config
- **WHEN** settings.json has `"experimental": {"teams": true}` but CLI is started without `--experimental-teams`
- **THEN** config value is used (teams enabled). When CLI explicitly passes `--experimental-teams=false`, teams are disabled regardless of config.

### Requirement: Create named teams
The system SHALL provide a TeamCreate tool that creates a named team. A team consists of a name, a member registry, a shared task list directory, and runtime coordination state (mailboxes, file locks, phase trackers). Team configs SHALL be persisted at `~/.glamdring/teams/{name}/config.json`. Task storage SHALL be persisted at `~/.glamdring/tasks/{name}/`.

#### Scenario: Create a new team
- **WHEN** the lead agent calls TeamCreate with name "backend-refactor"
- **THEN** the system creates the team config file, task directory, and in-memory team state, and the creating agent is registered as the team lead

#### Scenario: Duplicate team name
- **WHEN** TeamCreate is called with a name that already exists
- **THEN** the tool returns an error indicating the team already exists

### Requirement: Agent identity within teams
Each agent in a team SHALL have a unique name (string identifier) assigned at spawn time. The name SHALL be used for message routing, task ownership, and file lock attribution. The agent's name SHALL be available to the agent via its system prompt context.

#### Scenario: Agent knows its identity
- **WHEN** a team agent is spawned with name "auth-impl"
- **THEN** the agent's system prompt includes its name, team name, and role context

### Requirement: Team membership registration
When a subagent is spawned with a `team_name` parameter, the system SHALL register it as a member of that team. The team config SHALL track all active members with their name, agent type, and status (active, idle, shutdown).

#### Scenario: Agent joins team at spawn
- **WHEN** the Task tool is called with team_name "backend-refactor" and name "auth-impl"
- **THEN** the spawned agent is registered in the team's member list and can send/receive messages

### Requirement: Graceful shutdown protocol
The system SHALL support a shutdown protocol where the team lead sends a shutdown request to a specific agent. The target agent SHALL receive the request and respond with approval (triggering process exit) or rejection (with a reason). The lead SHALL be notified of the response.

#### Scenario: Agent approves shutdown
- **WHEN** the lead sends a shutdown request to agent "auth-impl" and the agent approves
- **THEN** the agent's goroutine terminates, its file locks are released, and its team membership status is set to "shutdown"

#### Scenario: Agent rejects shutdown
- **WHEN** the lead sends a shutdown request and the agent rejects with reason "still working on task #3"
- **THEN** the lead receives the rejection message and the agent continues running

### Requirement: Team deletion
The system SHALL provide a TeamDelete tool that removes a team's config directory and task directory. TeamDelete SHALL fail if the team has active (non-shutdown) members.

#### Scenario: Delete team with all members shut down
- **WHEN** all team members have been shut down and the lead calls TeamDelete
- **THEN** the team config, task directory, and in-memory state are cleaned up

#### Scenario: Delete team with active members
- **WHEN** TeamDelete is called while agents are still active
- **THEN** the tool returns an error listing the active members that must be shut down first

### Requirement: Automatic cleanup on agent termination
When a team agent's goroutine terminates (via shutdown, crash, or context cancellation), the system SHALL release all file locks held by that agent, update its membership status, and notify the team lead.

#### Scenario: Agent crashes mid-task
- **WHEN** a team agent's context is cancelled unexpectedly
- **THEN** the agent's file locks are released, its status is set to "shutdown", and the lead receives a notification with the error

### Requirement: Decomposed team subsystems
The TeamManager SHALL be a thin coordinator that composes focused subsystem interfaces. Each subsystem SHALL be independently testable and implement a defined Go interface. The subsystems SHALL be: MemberRegistry (join, leave, status), TaskStore (CRUD, dependencies, persistence), Mailbox (per-agent channels, delivery, wakeup), LockManager (acquire, release, query), ContextCache (set, get, list compacted summaries), PhaseTracker (current phase per agent, advance, gate state), and CheckinTracker (per-agent tool call counters).

#### Scenario: Subsystems are independently testable
- **WHEN** a test needs to verify file locking behavior
- **THEN** the test can instantiate a LockManager in isolation without creating a full TeamManager

#### Scenario: Cross-cutting cleanup via coordinator
- **WHEN** an agent shuts down
- **THEN** TeamManager coordinates cleanup across subsystems: LockManager.ReleaseAll(agentName), MemberRegistry.SetStatus(agentName, "shutdown"), Mailbox.Close(agentName)

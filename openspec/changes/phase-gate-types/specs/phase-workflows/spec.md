## MODIFIED Requirements

### Requirement: User-defined workflows
The system SHALL accept workflow definitions as an ordered list of phases, where each phase specifies: a name (string), a tool whitelist (list of tool names), a model tier (string, optional), a gate type (`auto`, `leader`, or `condition`; optional, defaults to `auto`), and a gate configuration (map of string key-value pairs; optional). Workflows SHALL be definable in settings.json under a `"workflows"` key, or inline when spawning a team agent. Workflow validation SHALL reject condition gates with no `command` in gate_config, and SHALL warn on unrecognized gate types.

#### Scenario: Custom two-phase workflow
- **WHEN** a user defines a workflow with phases ["explore", "build"] and the lead spawns an agent with that workflow
- **THEN** the agent starts in "explore" with the specified tool set and must call AdvancePhase to reach "build"

#### Scenario: Single-phase workflow
- **WHEN** a user defines a workflow with one phase ["implement"] that includes all tools
- **THEN** the agent starts in "implement" with no phase transitions needed and AdvancePhase is not registered

#### Scenario: Inline workflow at spawn time
- **WHEN** the lead spawns an agent with an inline workflow definition in the Task tool call
- **THEN** the inline workflow takes precedence over any named preset

#### Scenario: Custom workflow with leader gate
- **WHEN** a user defines a workflow with phase "plan" having gate "leader" and gate_config {"leader": "reviewer"}
- **THEN** the plan phase requires approval from agent "reviewer" before advancing

#### Scenario: Custom workflow with condition gate
- **WHEN** a user defines a workflow with phase "verify" having gate "condition" and gate_config {"command": "make test"}
- **THEN** the verify phase runs `make test` and only advances if it exits 0

#### Scenario: Condition gate missing command rejected
- **WHEN** a user defines a workflow with a condition gate but no command in gate_config
- **THEN** workflow validation returns an error at load time

### Requirement: Built-in workflow presets
The system SHALL provide built-in workflow presets as named compositions of phases. Presets SHALL include: `"rpiv"` (research/plan/implement/verify), `"plan-implement"` (2-phase with leader gate on plan), `"scoped"` (single work phase, no gates), and `"none"` (no enforcement). The default preset when no workflow is specified SHALL be `"scoped"` (Layer 2 enforcement without phases). The rpiv preset SHALL use a leader gate on the plan phase; all other phases SHALL use auto gates.

#### Scenario: RPIV preset with gates
- **WHEN** a team agent is spawned with workflow "rpiv"
- **THEN** the agent starts in the "research" phase with the following progression:
  - research (Read, Glob, Grep, Bash) -> gate=auto, model=haiku
  - plan (Read, Glob, Grep) -> gate=leader, model=sonnet
  - implement (Read, Write, Edit, Bash, Glob, Grep) -> gate=auto, model=opus
  - verify (Read, Bash, Glob, Grep) -> gate=auto, model=sonnet

#### Scenario: Plan-implement preset with leader gate
- **WHEN** a team agent is spawned with workflow "plan-implement"
- **THEN** the plan phase has gate=leader and the implement phase has gate=auto

#### Scenario: No-enforcement preset
- **WHEN** the lead spawns an agent with workflow "none"
- **THEN** the agent has no phase restrictions, no AdvancePhase tool, and all tools are available immediately

### Requirement: AdvancePhase tool
The system SHALL provide an AdvancePhase tool that agents call to signal completion of their current phase and request transition to the next phase. The tool SHALL include a required `summary` parameter where the agent describes what it accomplished in the current phase. The tool SHALL evaluate the current phase's gate type before advancing: auto gates advance immediately, leader gates block for approval, and condition gates execute a command.

#### Scenario: Auto-advance gate
- **WHEN** an agent calls AdvancePhase with a summary and the current phase gate is `auto`
- **THEN** the tool returns immediately with the new phase name, updated tool list, and previous phase name

#### Scenario: Leader approval gate blocks
- **WHEN** an agent calls AdvancePhase with a summary and the current phase gate is `leader`
- **THEN** the tool sends a phase approval request to the team lead and blocks until the lead responds

#### Scenario: Condition gate with passing condition
- **WHEN** an agent calls AdvancePhase with a summary, the gate is `condition` with command `go test ./...`, and the command exits 0
- **THEN** the tool advances the phase and returns the new phase details

#### Scenario: Condition gate with failing condition
- **WHEN** an agent calls AdvancePhase with a summary, the gate is `condition`, and the condition check fails
- **THEN** the tool returns an error with the failure details and the agent remains in the current phase

### Requirement: Phase state visibility
The AdvancePhase tool description and the agent's system prompt SHALL clearly communicate the current phase, available tools, and what gate governs the next transition. The agent SHALL always know what phase it is in, what tools it can use, and what gate type controls the next advancement.

#### Scenario: Agent system prompt includes phase and gate context
- **WHEN** a team agent starts a turn in the "plan" phase with gate "leader"
- **THEN** the system prompt or injected context includes: current phase name, available tools, gate type for advancing (leader -- requires lead approval), and the overall workflow progression

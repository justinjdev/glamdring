## ADDED Requirements

### Requirement: Phase-locked tool registries
Team agents SHALL receive tools through a PhaseRegistry that filters available tools based on the agent's current workflow phase. The PhaseRegistry SHALL implement the same interface as the standard Registry but only return tools whitelisted for the current phase. Tools not in the current phase SHALL NOT appear in the API request's tools array.

#### Scenario: Research phase excludes write tools
- **WHEN** a team agent is in the "research" phase and the system builds the API request
- **THEN** the tools array contains only Read, Glob, Grep, SendMessage, TaskUpdate, and AdvancePhase -- Edit, Write, and Bash are absent

#### Scenario: Implement phase includes write tools
- **WHEN** a team agent advances to the "implement" phase
- **THEN** the tools array adds Edit, Write, and Bash (scoped) alongside the always-available tools

#### Scenario: Model cannot call excluded tools
- **WHEN** the API response contains a tool_use block for a tool not in the current phase
- **THEN** the tool dispatch returns an error result to the model (defensive fallback, should not happen with correct schema filtering)

### Requirement: User-defined workflows
The system SHALL accept workflow definitions as an ordered list of phases, where each phase specifies: a name (string), a tool whitelist (list of tool names), a model tier (string, optional), and a gate type (AutoAdvance, LeaderApproval, or Condition). Workflows SHALL be definable in settings.json under a `"workflows"` key, or inline when spawning a team agent.

#### Scenario: Custom two-phase workflow
- **WHEN** a user defines a workflow with phases ["explore", "build"] and the lead spawns an agent with that workflow
- **THEN** the agent starts in "explore" with the specified tool set and must call AdvancePhase to reach "build"

#### Scenario: Single-phase workflow
- **WHEN** a user defines a workflow with one phase ["implement"] that includes all tools
- **THEN** the agent starts in "implement" with no phase transitions needed and AdvancePhase is not registered

#### Scenario: Inline workflow at spawn time
- **WHEN** the lead spawns an agent with an inline workflow definition in the Task tool call
- **THEN** the inline workflow takes precedence over any named preset

### Requirement: Built-in workflow presets
The system SHALL provide built-in workflow presets as named compositions of phases. Presets SHALL include: `"rpiv"` (research/plan/implement/verify), `"plan-implement"` (2-phase with leader gate), `"scoped-only"` (no phases, enforcement via scoped tools only), and `"none"` (no enforcement). The default preset when no workflow is specified SHALL be `"rpiv"`.

#### Scenario: Default workflow (rpiv preset)
- **WHEN** a team agent is spawned without specifying a workflow
- **THEN** the agent starts in the "research" phase with the rpiv progression:
  - research (Read, Glob, Grep, SendMessage, TaskUpdate, AdvancePhase) -> AutoAdvance, model=haiku
  - plan (Read, Glob, Grep, SendMessage, TaskUpdate, AdvancePhase) -> LeaderApproval, model=sonnet
  - implement (Read, Glob, Grep, Edit, Write, Bash, SendMessage, TaskUpdate, AdvancePhase) -> AutoAdvance, model=opus
  - verify (Read, Glob, Grep, Bash, SendMessage, TaskUpdate, AdvancePhase) -> AutoAdvance, model=sonnet

#### Scenario: No-enforcement preset
- **WHEN** the lead spawns an agent with workflow "none"
- **THEN** the agent has no phase restrictions, no AdvancePhase tool, and all tools are available immediately (Layer 1 coordination only)

### Requirement: AdvancePhase tool
The system SHALL provide an AdvancePhase tool that agents call to signal completion of their current phase and request transition to the next phase. The tool SHALL include a required summary parameter where the agent describes what it accomplished in the current phase.

#### Scenario: Auto-advance gate
- **WHEN** an agent calls AdvancePhase and the current phase gate is AutoAdvance
- **THEN** the tool returns immediately with the new phase name and updated tool list

#### Scenario: Leader approval gate blocks
- **WHEN** an agent calls AdvancePhase and the current phase gate is LeaderApproval
- **THEN** the tool sends a phase approval request to the team lead and blocks until the lead responds

#### Scenario: Condition gate with passing condition
- **WHEN** an agent calls AdvancePhase, the gate is Condition with a "tests pass" check, and `go test ./...` exits 0
- **THEN** the tool advances the phase

#### Scenario: Condition gate with failing condition
- **WHEN** an agent calls AdvancePhase, the gate is Condition, and the condition check fails
- **THEN** the tool returns an error with the failure details and the agent remains in the current phase

### Requirement: Phase advancement is sequential
Agents SHALL only advance one phase at a time. There SHALL be no mechanism to skip phases. An agent in "research" MUST advance through "plan" before reaching "implement".

#### Scenario: Cannot skip phases
- **WHEN** an agent in "research" phase calls AdvancePhase
- **THEN** the agent moves to "plan", not "implement" -- regardless of what the agent requests

### Requirement: Read-only tools available in all phases
Read, Glob, Grep, SendMessage, TaskUpdate, and AdvancePhase SHALL be available in every phase. Only write/execute tools (Edit, Write, Bash) SHALL be phase-restricted.

#### Scenario: Grep works in research phase
- **WHEN** an agent in "research" phase calls Grep
- **THEN** the tool executes normally

#### Scenario: Grep works in verify phase
- **WHEN** an agent in "verify" phase calls Grep
- **THEN** the tool executes normally

### Requirement: Phase-specific model selection
Each workflow phase SHALL specify a model tier that the agent loop uses when building API requests. The phase model SHALL override the session's default model for the duration of that phase. The default model assignments SHALL be: research=haiku, plan=sonnet, implement=opus, verify=sonnet. The model assignment SHALL be configurable per phase when defining custom workflows.

#### Scenario: Research phase uses Haiku
- **WHEN** a team agent is in the "research" phase and the system builds the API request
- **THEN** the request uses Haiku as the model, regardless of the session's default model setting

#### Scenario: Implement phase uses Opus
- **WHEN** a team agent advances to the "implement" phase
- **THEN** subsequent API requests use Opus as the model

#### Scenario: Model changes at phase transition
- **WHEN** an agent advances from "plan" (Sonnet) to "implement" (Opus)
- **THEN** the next API request uses Opus and the phase context reflects the model change

#### Scenario: Custom model override
- **WHEN** a workflow defines the plan phase with model "opus" instead of the default "sonnet"
- **THEN** the plan phase uses Opus

### Requirement: Model fallback chain
Each workflow phase SHALL support an optional fallback model. When the primary model returns a rate limit (429) or server error (5xx), the agent loop SHALL retry with the fallback model. The default fallback assignments SHALL be: research (haiku -> sonnet), plan (sonnet -> opus), implement (opus -> sonnet), verify (sonnet -> opus). The fallback SHALL be configurable per phase in custom workflow definitions.

#### Scenario: Primary model rate-limited
- **WHEN** an agent in the "research" phase sends an API request and Haiku returns a 429 rate limit error
- **THEN** the agent loop retries the request using Sonnet (the fallback model) without requiring phase advancement or user intervention

#### Scenario: Fallback model also fails
- **WHEN** both the primary and fallback models return errors
- **THEN** the agent loop surfaces the error to the team lead via SendMessage and pauses (does not crash)

#### Scenario: Custom fallback override
- **WHEN** a workflow defines the research phase with model "haiku" and fallback "opus"
- **THEN** the research phase falls back to Opus instead of the default Sonnet

### Requirement: Phase transition context compaction
When an agent advances to a new phase, the system SHALL compact the previous phase's conversation history into a structured summary. The new phase SHALL start with a clean conversation containing: the compacted summary as a user message, the current task details, and the phase-appropriate system prompt. The compaction SHALL be performed by a Haiku API call to minimize cost.

#### Scenario: Research-to-plan compaction
- **WHEN** an agent advances from "research" to "plan" and the research conversation contains 50K tokens of file reads and analysis
- **THEN** the conversation is compacted into a structured summary (target: 2-3K tokens), and the plan phase starts with only the summary plus task context

#### Scenario: Compaction preserves key findings
- **WHEN** the research phase identified 5 relevant files and 3 architectural constraints
- **THEN** the compacted summary includes the file paths, their purposes, and the constraints -- sufficient for the plan phase to proceed without re-reading files

#### Scenario: Compaction across all transitions
- **WHEN** an agent advances from any phase to any subsequent phase
- **THEN** context compaction occurs at the boundary, preventing unbounded context growth

### Requirement: Shared context cache
When context compaction occurs at a phase boundary, the compacted output SHALL be stored in the TeamManager's shared context cache under a key derived from agent name and phase (e.g., `"researcher:research"`). The lead SHALL be able to inject cached context entries into new agents at spawn time via an `inject_context` parameter, and optionally specify a `start_phase` to skip earlier phases when sufficient context is provided.

#### Scenario: Context cached on phase advance
- **WHEN** agent "researcher" advances from research to plan in team "backend-refactor"
- **THEN** the compacted summary is stored in the shared context cache under key "researcher:research"

#### Scenario: New agent inherits context
- **WHEN** the lead spawns agent "impl-B" with inject_context=["researcher:research"] and start_phase="plan"
- **THEN** agent "impl-B" starts in the plan phase with the cached context injected as an initial user message

#### Scenario: Missing context key is skipped
- **WHEN** the lead spawns an agent with inject_context=["nonexistent:key"]
- **THEN** the missing key is ignored and the agent starts normally without that context

#### Scenario: Multiple context entries injected
- **WHEN** the lead spawns an agent with inject_context=["researcher:research", "architect:plan"]
- **THEN** both cached entries are injected as context, concatenated in the specified order

#### Scenario: Shared context eliminates redundant work
- **WHEN** the lead spawns 3 agents with inject_context=["researcher:research"] and start_phase="plan"
- **THEN** all 3 agents skip the research phase and begin planning with the same shared findings

### Requirement: Phase state visibility
The AdvancePhase tool description and the agent's system prompt SHALL clearly communicate the current phase, available tools, and what gate governs the next transition. The agent SHALL always know what phase it is in and what tools it can use.

#### Scenario: Agent system prompt includes phase context
- **WHEN** a team agent starts a turn in the "plan" phase
- **THEN** the system prompt or injected context includes: current phase name, available tools, gate type for advancing, and the overall workflow progression

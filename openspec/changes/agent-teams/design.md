## Context

Glamdring has a working subagent system: the Task tool spawns isolated agent loops (goroutines) that run to completion and return results. Each subagent gets its own conversation, tool set, and system prompt. Subagents share the filesystem but have no post-spawn communication with the parent or each other.

This works for independent, parallelizable tasks (research, exploration). It fails for coordinated work where multiple agents need to: share a task list, communicate progress, avoid file conflicts, and follow a structured workflow. The root cause of coordination failures in existing multi-agent systems is that workflow enforcement lives in the system prompt -- agents can and do ignore it.

The existing codebase provides several integration points:
- `SubagentRunner` callback pattern in Task tool (injects team context at spawn)
- Dynamic tool filtering via `SubagentOptions.Tools` (restrict tools per agent)
- `PermissionRequest` blocking channel pattern (reusable for phase gates)
- Tool interface with `Execute(ctx, input) (Result, error)` (wrappable via decorator)

## Goals / Non-Goals

**Goals:**
- Enable multi-agent coordination with shared task lists, messaging, and lifecycle management
- Structurally enforce workflow phases so agents cannot skip, combine, or self-approve transitions
- Structurally enforce file scope so agents cannot write outside their assigned area
- Prevent file conflicts between concurrent agents via locking
- Force agents to communicate progress via mandatory check-ins
- Preserve existing single-agent behavior -- teams are fully opt-in

**Non-Goals:**
- Distributed agents across processes or machines (agents are goroutines in a single process)
- Persistent teams across sessions (teams exist for one session, tasks persist as files)
- Role-based access control beyond phase/scope (no "admin" vs "member" agent tiers)
- Agent-to-agent direct tool delegation (agents communicate via messages, not shared tool calls)
- MCP-based workflow plugins (future extension -- MCP servers could register custom workflows, gate conditions, and compaction strategies; config-based workflow definitions ship first)

## Decisions

### 1. Phase enforcement via filtered tool schemas (not runtime checks)

**Decision:** Phase enforcement happens at the API schema level. When building the MessageRequest for a team agent, only tools in the current phase are included in the `tools` array. The model cannot generate tool_use blocks for tools not in the schema.

**Alternative considered:** Runtime checks in Execute() that reject out-of-phase tools. Rejected because the model can still attempt the call (wasting a turn) and may persist in trying. Excluding tools from the schema is invisible and absolute -- the model doesn't know the tool exists.

**Implementation:** A `PhaseRegistry` wraps the real `Registry`. It implements the same `Schemas()` and `Get()` methods but filters by the current phase's tool whitelist. The agent loop uses PhaseRegistry instead of Registry when the agent is in a team workflow.

### 2. Phase transitions via blocking AdvancePhase tool

**Decision:** Agents advance phases by calling an `AdvancePhase` tool. For `GateLeaderApproval` gates, this tool blocks on a Go channel until the team lead sends approval. This reuses the same blocking pattern as `PermissionRequest`.

**Alternative considered:** Automatic phase detection (infer phase from tool usage patterns). Rejected because it's unreliable and removes the explicit gate. The agent must consciously signal "I'm done with this phase."

**Alternative considered:** Phase advancement via TaskUpdate (overload existing tool). Rejected because phase transitions need distinct semantics (blocking, approval flow) that don't fit TaskUpdate's CRUD model.

### 3. Scoped tools via decorator pattern

**Decision:** Scoped enforcement uses the decorator pattern. `ScopedWrite`, `ScopedEdit`, `ScopedBash` wrap the real tools, check scope rules in Execute() before delegating. Decorators are composable: `CheckinGate(ScopedEdit(Edit))`.

**Alternative considered:** Modify the base tools to accept scope parameters. Rejected because it couples scope logic to tools that work fine without it, and non-team agents shouldn't pay the complexity cost.

**Decorator chain for team agents:**
```
CheckinGate -> FileLock -> ScopedTool -> BaseTool
```
- CheckinGate: enforces mandatory progress reporting
- FileLock: acquires/checks file locks before writes
- ScopedTool: validates paths/commands against task scope
- BaseTool: the real Edit/Write/Bash implementation

### 4. Team state: in-memory with file-backed tasks

**Decision:** Team membership, file locks, phase state, check-in counters, and mailboxes are in-memory (managed by `TeamManager`). Task lists are persisted as JSON files for durability. If the process crashes, tasks survive but team runtime state resets (acceptable -- teams are session-scoped).

**Alternative considered:** Full persistence for all state. Rejected because file locks and phase state are meaningless after a crash (agents are gone). Task persistence is valuable because it's the durable record of work.

### 5. Messaging via buffered Go channels

**Decision:** Each team agent gets a mailbox (buffered channel). `SendMessage` writes to the recipient's channel. The agent loop checks for incoming messages between turns (after each `runTurn` completes). Phase approval uses a separate dedicated channel to avoid message ordering issues.

**Alternative considered:** File-based message queues. Rejected because agents are goroutines in the same process -- channels are simpler, faster, and naturally concurrent-safe.

**Message delivery timing:** Messages are delivered between turns (not mid-turn). This is simpler and avoids interrupting an agent mid-thought. The agent sees new messages as injected user-role messages at the start of its next iteration.

### 6. Three-layer composable architecture

**Decision:** The team system is organized as three independent layers. Each layer is opt-in and usable without the layers above it.

**Layer 1: Team primitives (coordination)**
- Team lifecycle (create, membership, shutdown)
- Task management (CRUD, dependencies, ownership, scope metadata)
- Messaging (DMs, broadcasts, shutdown protocol, approval flows)

These are always available when teams are enabled. No opinions about how agents work. A user could use just Layer 1 for lightweight coordination -- shared task list, messaging between agents, no enforcement at all.

**Layer 2: Enforcement primitives (composable, independent of each other)**
- PhaseRegistry -- accepts any `[]Phase` definition, not hardcoded
- Scoped tool decorators -- usable with or without phases (can attach to a task's scope metadata directly)
- File locking -- usable with or without phases
- Check-in enforcement -- usable with or without phases
- Model selection -- per phase or per agent (if no phases, set on agent spawn)
- Context compaction -- at phase boundaries or on demand

Each primitive can be composed independently. You can use file locking + scoped tools without any phases. You can use phases without scoped tools. The decorator chain (CheckinGate -> FileLock -> ScopedTool -> BaseTool) composes from whatever subset is enabled.

**Layer 3: Workflow presets (convenience compositions)**
Presets are named configurations that compose Layer 2 primitives into complete workflows. They're shortcuts, not the system.

Built-in presets:
- `"rpiv"` -- 4-phase research/plan/implement/verify (the full workflow designed in this doc)
- `"plan-implement"` -- 2-phase with LeaderApproval gate at plan->implement
- `"scoped-only"` -- no phases, just scoped tools + file locking + check-ins
- `"none"` -- Layer 1 only, no enforcement (coordination without guardrails)

User-defined workflows via config:
```json
{
  "workflows": {
    "my-workflow": {
      "phases": [
        {"name": "explore", "tools": ["Read", "Glob", "Grep"], "model": "haiku", "gate": "auto"},
        {"name": "build", "tools": ["Read", "Glob", "Grep", "Edit", "Write", "Bash"], "model": "opus", "gate": "leader"}
      ]
    }
  }
}
```

When spawning a team agent, the lead specifies a workflow by name (preset or custom). If omitted, defaults to `"rpiv"`.

**Alternative considered:** Hardcoded 4-phase RPIV as the only workflow. Rejected because it ties users to a specific process. The primitives are inherently generic -- phases are just `{name, tools, model, gate}` tuples. Hardcoding one arrangement wastes the composability.

**Alternative considered:** No presets, require users to always define custom workflows. Rejected because most users want reasonable defaults. Presets provide good defaults while keeping the escape hatch open.

### 7. Experimental feature flag

**Decision:** All team functionality is gated behind an experimental flag. When disabled (the default), team tools (TeamCreate, TeamDelete, TaskCreate, TaskList, TaskGet, TaskUpdate, SendMessage, AdvancePhase) are not registered in the tool registry and the Task tool ignores `team_name`/`name` parameters. Two activation paths:

- **CLI flag:** `--experimental-teams` enables teams for the current session.
- **Config:** `"experimental": {"teams": true}` in settings.json enables teams persistently (per-project or user-level).
- CLI flag overrides config. Both default to false.

**Rationale:** Teams are a significant new capability with novel enforcement patterns. Gating behind an explicit opt-in protects existing users from unexpected behavior, lets us iterate on the design without stability commitments, and makes the blast radius of bugs zero for non-opt-in users.

### 8. Phase-specific model selection

**Decision:** Each workflow phase specifies a model tier. The agent loop uses the phase's model when building API requests, overriding the session default. Default model assignments:

```
research: haiku    ($0.80/$4 per M tokens)
plan:     sonnet   ($3/$15 per M tokens)
implement: opus    ($15/$75 per M tokens)
verify:   sonnet   ($3/$15 per M tokens)
```

**Rationale:** Research is grep/read/glob -- Haiku handles file navigation and pattern matching well. Planning needs more reasoning but not Opus-level. Only implementation genuinely benefits from the strongest model. Verification needs to understand code but mostly runs tests and checks output.

If agents spend ~30% of tokens on research, ~15% on planning, ~40% on implementation, ~15% on verification, the weighted cost is roughly 40% of all-Opus pricing. For a 4-agent team, this alone turns ~7x cost into ~3x.

**Implementation:** Add `Model string` field to the `Phase` struct. The agent loop reads `workflow.CurrentPhase().Model` when building the API request. If empty, falls back to the session's configured model.

**Alternative considered:** Same model throughout, rely on context compaction alone to reduce costs. Rejected because model selection is the single largest cost lever and requires minimal implementation effort.

### 9. Phase transition context compaction

**Decision:** When an agent advances to a new phase, the system compacts the previous conversation into a structured summary. The new phase starts with a clean context containing: the compacted summary, current task details, and the phase-appropriate system prompt. This prevents unbounded context growth across phases.

**Implementation:** At phase transition (in AdvancePhase tool, after gate clears):
1. Take the agent's current conversation history.
2. Send a compaction request to Haiku: "Summarize the findings from this conversation as structured context for the next phase."
3. Replace the conversation history with a single user message containing the compacted summary.
4. Continue with the new phase.

**Cost of compaction itself:** One Haiku API call per phase transition. Trivial compared to the savings from smaller context in subsequent phases. A research phase that accumulated 50K tokens of file reads becomes a 2-3K summary. Every subsequent API call in plan/implement/verify phases sends dramatically less input.

**Alternative considered:** No compaction, let context accumulate. Rejected because context growth is the second largest cost driver. A 4-phase agent with no compaction sends the full research conversation on every plan/implement/verify API call.

**Alternative considered:** Manual compaction via /compact command. Rejected because compaction at phase boundaries is the natural point and shouldn't require agent initiative (which agents may skip).

### 10. Shared context cache

**Decision:** Teams have a shared context cache -- a key-value store of compacted summaries that any agent can contribute to and any agent can receive at spawn time. When context compaction occurs at a phase boundary, the compacted output is stored in the cache under a key derived from the agent name and phase (e.g., `"researcher:research"`, `"auth-impl:plan"`).

**Implementation:**
- `TeamManager.SetContext(teamName, key, content string)` -- called during phase compaction or explicitly by agents.
- `TeamManager.GetContext(teamName, key string) string` -- called when spawning new team agents.
- `TeamManager.ListContextKeys(teamName string) []string` -- lets the lead see what's available.
- When spawning a new team agent, the lead can specify `inject_context: ["researcher:research"]` to inject specific cached context. The agent can also specify `start_phase: "plan"` to skip earlier phases when context is injected.

**Rationale:** Eliminates redundant work across agents. The common case is sharing research findings, but the mechanism is generic -- a planning agent's output could be injected into an implementation agent, or one implementation agent's findings could help another. The cache stores compacted summaries (not raw conversation), so injected context is already token-efficient.

**Alternative considered:** Research-specific cache with `skip_research: true` shorthand. Rejected as too narrow -- the underlying need is sharing compacted context between agents, not specifically research findings. A generic cache supports the research case and any other sharing pattern.

### 11. File locking granularity: per-file, not per-directory

**Decision:** Locks are acquired per file path when an agent successfully writes/edits. The lock is held until the agent's task completes or the agent is shut down.

**Alternative considered:** Per-directory locking (lock `pkg/auth/` when any file in it is touched). Rejected because it's too coarse -- two agents could reasonably work on different files in the same package.

**Alternative considered:** Advisory locks (warn but don't block). Rejected because the whole point is structural enforcement. Warnings are soft constraints that agents ignore.

## Risks / Trade-offs

**[Risk] Leader becomes a bottleneck** -- If all agents hit GateLeaderApproval simultaneously, the lead must review and approve each one sequentially.
-> Mitigation: Default workflow only gates plan->implement. Research and verify are auto-advance. Leaders can configure per-task gate types. Future: allow peer approval (another agent can approve, not just the lead).

**[Risk] Phase tool sets are too restrictive** -- An agent in "implement" phase might need to do additional research (Grep/Read) that would be blocked if we're too aggressive with tool filtering.
-> Mitigation: Read-only tools (Read, Glob, Grep) are available in ALL phases. Only write/execute tools are phase-restricted. SendMessage and TaskUpdate are always available.

**[Risk] File locking causes deadlocks** -- Agent A holds file X, needs file Y. Agent B holds file Y, needs file X.
-> Mitigation: File locks are per-agent-per-task, not per-operation. An agent can acquire multiple locks. Deadlocks would require circular task dependencies, which the task dependency system already prevents. If an agent can't acquire a lock, it gets a clear error with the owner's name and can message them.

**[Risk] Check-in enforcement is annoying** -- Agents might spend their check-in on a meaningless "still working" message just to clear the counter.
-> Mitigation: The counter threshold is tunable (default 15). The check-in must be a TaskUpdate (with actual status) or a SendMessage with content, not just a ping. Future: validate check-in quality heuristically.

**[Risk] Scoped Bash is hard to get right** -- Command restriction via pattern matching is inherently fragile (shell escaping, pipes, subshells).
-> Mitigation: Default to excluding Bash entirely from research/plan phases. In implement phase, Bash is scoped by allow-list patterns (e.g., `go test*`, `go build*`). Deny patterns catch obvious dangerous commands. Accept that determined circumvention is possible via Bash -- the goal is preventing accidental scope violations, not adversarial agents.

**[Trade-off] Complexity vs. enforcement** -- The decorator chain (CheckinGate -> FileLock -> ScopedTool -> BaseTool) adds layers. Each team agent tool call traverses 3-4 wrappers before reaching the real implementation.
-> Acceptable: The wrapper checks are O(1) (map lookups, counter checks). The complexity is in setup (composing wrappers at spawn time), not in per-call overhead. Non-team agents are unaffected.

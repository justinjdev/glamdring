## Why

Glamdring supports subagents (isolated, fire-and-forget tasks) but has no infrastructure for coordinated multi-agent work -- teams of agents that share task lists, communicate, and work on different parts of a problem concurrently. Existing tools (e.g., Claude Code's fellowship) attempt coordination through prompt instructions, but agents routinely ignore soft constraints: skipping research phases, auto-approving their own work, combining phases, and editing files outside their scope. Glamdring should solve this structurally by making phase gates, scope enforcement, and coordination primitives first-class concepts enforced in Go code rather than system prompt suggestions.

## What Changes

- New team lifecycle: create named teams with shared task lists, spawn named agents that join teams, shut down teams cleanly
- New task management: persistent task CRUD (create, list, get, update) with status, ownership, dependencies, and scope metadata
- New inter-agent messaging: direct messages, broadcasts, shutdown protocol, and phase approval flows
- Phase-locked tool registries: agents receive different tool sets depending on their current workflow phase (research, plan, implement, verify); phase transitions are gated by leader approval or conditions, enforced at the tool registry level -- agents literally cannot call tools outside their phase
- Cost optimization via phase-specific models (Haiku for research, Sonnet for planning/verify, Opus for implementation), automatic context compaction at phase boundaries, and shared research cache to eliminate redundant codebase exploration across agents -- targeting ~3x total cost vs. Claude Code's ~7x
- Scoped tool wrappers: Write/Edit/Bash tools are wrapped with path and command restrictions derived from the agent's assigned task scope; enforcement happens in Go code before tool execution
- File locking: when an agent modifies a file, it's locked to that agent; other agents get clear errors if they try to modify locked files
- Mandatory check-ins: after N tool calls without a SendMessage or TaskUpdate, further tool calls are blocked until the agent reports progress
- Team-aware Task tool: extend the existing Task tool with `team_name` and `name` parameters so spawned subagents join teams and can be messaged by name
- Experimental feature gate: all team functionality behind `--experimental-teams` flag / config setting, default off

## Capabilities

### New Capabilities
- `team-lifecycle`: Team creation, membership, shutdown protocol, and agent identity within teams
- `task-management`: Persistent task CRUD with status, ownership, dependencies, scope, and turn budgets
- `agent-messaging`: Inter-agent direct messages, broadcasts, shutdown requests/responses, and phase approval flows
- `phase-workflows`: Composable phase system -- user-defined or preset workflows (rpiv, plan-implement, scoped-only, none) with phase-locked tool registries, gated transitions, phase-specific model selection, context compaction at phase boundaries, and shared context cache between agents
- `scoped-tools`: Tool wrappers that enforce file path restrictions, command restrictions, file locking, and mandatory check-ins at the Go code level

### Modified Capabilities
- `subagents`: Extend Task tool with team_name and name parameters; spawned agents optionally join teams with phase workflows and scoped tools
- `tool-system`: Add support for tool wrappers (decorators) that intercept Execute calls for scope checking, file locking, and check-in enforcement
- `permission-system`: Integrate team-level scope rules alongside existing three-tier permission model

## Impact

- **New packages:** `pkg/teams/` (team state, task storage, mailbox, file locks, phase workflows), new tool files in `pkg/tools/` (team_create, team_delete, task_create, task_list, task_get, task_update, send_message, advance_phase)
- **Modified packages:** `pkg/tools/task.go` (team-aware spawning), `pkg/tools/registry.go` (dynamic tool set swapping for phases), `pkg/agent/session.go` (agent identity, team membership, phase state)
- **New config:** Team configs at `~/.glamdring/teams/{name}/config.json`, task storage at `~/.glamdring/tasks/{name}/`
- **Dependencies:** Builds on existing subagent infrastructure (Chunk 1 + 7), permission system, tool interface, and hooks
- **No breaking changes** to existing single-agent workflows; teams are opt-in when the Task tool is called with team parameters

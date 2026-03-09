## Context

glamdring renders agent output as a sequence of typed blocks (`blockText`, `blockToolCall`, `blockToolResult`, etc.) in `internal/tui/output.go`. Block types are an enum; rendering is a switch statement in `doRender()`. New block types follow the established pattern.

Agent messages flow: `agent.Message` → `AgentMsg` wrapper → `handleAgentMsg()` in `model.go` → `OutputModel` methods. The `MessageToolCall` case currently has a special-case switch for file-modifying tools (`Edit`, `Write`, `Bash`). The same mechanism is the right place to intercept `TodoWrite`.

Claude sends `TodoWrite` as a standard tool call with `ToolInput["todos"]` containing an array of task objects: `{"id": string, "content": string, "status": "pending"|"in_progress"|"completed"}`. The tool result is always a trivial acknowledgment and carries no user-visible information.

## Goals / Non-Goals

**Goals:**
- Detect `TodoWrite` tool calls and render them as a live task list block, not as a generic tool call/result pair
- Update the task list block in-place when subsequent `TodoWrite` calls arrive in the same turn
- Finalize (freeze) the task list when the turn completes (`MessageDone`)
- Suppress the `TodoWrite` tool result block (it adds noise with no value)

**Non-Goals:**
- Animating individual task rows with per-frame spinners (static status indicators are sufficient)
- Persisting the task list across turns
- Any interaction with the multi-agent team task board (`pkg/teams/`)
- Adding a keyboard shortcut to expand/collapse the task list (no evidence it's needed yet)

## Decisions

### 1. New `blockTaskList` block type

Add `blockTaskList` to the `blockKind` enum alongside the existing types. The block stores a `[]todoItem` slice directly on `outputBlock` via a new `tasks []todoItem` field, keeping content/rendering separate.

**Why not reuse `blockToolCall`:** The task list has different semantics — it's mutable (updated across multiple calls) and uses a custom multi-line render. Reusing `blockToolCall` would require forking its rendering path with special-case logic. A distinct type is cleaner and consistent with the existing pattern (thinking blocks got their own type for the same reason).

**Why store `[]todoItem` on the block instead of serializing to `content` string:** The render function needs structured data to apply per-status styling. Storing raw structs avoids a parse round-trip on every render.

### 2. Track the active task list block index on `OutputModel`

Add `taskListIdx int` (default `-1`) to `OutputModel`. When a new `TodoWrite` call arrives:
- If `taskListIdx == -1`: append a new `blockTaskList` block and record its index
- If `taskListIdx >= 0`: update `m.blocks[taskListIdx].tasks` in-place and invalidate its render cache

Reset `taskListIdx` to `-1` when the turn ends (`FinalizeTaskList()` called from `MessageDone`).

**Why not look up by scanning blocks:** The `taskListIdx` field is O(1) and avoids iterating blocks on every update. Consistent with how `lastToolCallIndex()` is already used for the spinner.

**Why reset per-turn:** A new agent turn should start a fresh task list. Keeping the old one visible but frozen (finalized) is the desired behavior — the reset only affects the index, not the rendered block.

### 3. Intercept in `handleAgentMsg()`, not in `AppendToolCall()`

The `MessageToolCall` case in `model.go` already has a tool-name switch. Add `"TodoWrite"` there and call a new `OutputModel.UpdateTaskList(todos []todoItem)` method instead of `AppendToolCall`.

For `MessageToolResult` with a preceding `TodoWrite` call, track whether the last tool was `TodoWrite` on the model (a `lastToolWasTodo bool` flag) and skip `AppendToolResult` if set.

**Why not filter inside `AppendToolCall`/`AppendToolResult`:** Those methods don't know the tool name in context — they receive already-processed strings. Filtering at the routing layer in `model.go` keeps the OutputModel methods dumb and composable.

### 4. Static status indicators

Render task status as:
- `[ ]` — pending
- `[>]` — in_progress
- `[x]` — completed

No per-frame animation on task rows. The existing tool spinner still shows on the last `blockToolCall` (if any) during tool execution; the task list itself does not animate.

**Why not animate:** Per-frame re-renders of `blockTaskList` would require adding it to the `skipCache` path in `doRender()`, coupling the render loop to task state. The visual gain is minimal — the list updates when new `TodoWrite` calls arrive, which is the meaningful signal.

**Considered:** blinking dot on the active task (Claude Code style). Rejected for now as it requires hooking the spinner tick into the task list block, adding complexity with low user-visible benefit at this stage.

## Risks / Trade-offs

- **TodoWrite schema changes**: The `todos` array structure is assumed based on Claude Code's implementation. If the model emits a different schema, parsing will fail silently (empty task list). Mitigation: defensive parsing that ignores malformed entries.
- **Multiple task lists per turn**: If the agent calls `TodoWrite` mid-turn and then again later, we update in-place (intended). If the agent abandons a task list and starts a new one after several unrelated tool calls, we still update the same block. This is the correct behavior — one task list per turn.
- **`lastToolWasTodo` flag**: A boolean flag is fragile if tool calls interleave unexpectedly. However, `MessageToolResult` always follows its corresponding `MessageToolCall` in the stream, so the pairing is reliable.

## Migration Plan

No migration required. Feature is purely additive — new block type, new intercept path. Existing tool call/result rendering is unchanged for all other tools.

## Open Questions

- Should the task list block be collapsible (like large tool results)? Deferred — task lists are typically short.
- Should a finalized task list show a summary line (e.g., "3/4 tasks completed")? Deferred — spec it if requested.

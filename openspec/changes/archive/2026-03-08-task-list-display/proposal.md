## Why

Long agent turns involving many tool calls are opaque to the user — there is no high-level view of what the agent is working through or how far along it is. Claude Code addresses this with an inline task list UI that tracks structured progress (via TodoWrite tool calls). glamdring has no equivalent, making complex turns feel like an undifferentiated stream of events.

## What Changes

- Intercept `ToolCall` messages with tool name `TodoWrite` in the TUI output model
- Parse the todo list payload (array of tasks with id, content, and status fields)
- Render a live task list block inline in the conversation output, distinct from regular tool call blocks
- Update the task list in-place as new `TodoWrite` calls arrive during the same turn
- Collapse or freeze the task list when the turn completes (no further updates)

## Capabilities

### New Capabilities
- `task-list-display`: Live task list UI rendered from TodoWrite tool calls during agent turns. Shows pending/in-progress/completed status per task with inline updates.

### Modified Capabilities
- `tui`: Tool call rendering gains a special case for TodoWrite — instead of the generic tool call block, a task list block is shown and updated.

## Impact

- `internal/tui/output.go`: New block type `blockTaskList`; intercept and render TodoWrite calls
- `internal/tui/model.go`: Route TodoWrite `ToolCall` messages to task list update logic
- `pkg/agent/message.go`: No changes required — existing `ToolCall` message type carries the data
- No new dependencies
- No breaking changes

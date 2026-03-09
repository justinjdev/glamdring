## 1. Data Types

- [x] 1.1 Define `todoItem` struct with `ID`, `Content`, `Status` string fields in `internal/tui/`
- [x] 1.2 Add `blockTaskList` to the `blockKind` enum in `output.go`
- [x] 1.3 Add `tasks []todoItem` field to `outputBlock` struct

## 2. OutputModel Changes

- [x] 2.1 Add `taskListIdx int` field (initialized to `-1`) to `OutputModel`
- [x] 2.2 Implement `UpdateTaskList(todos []todoItem)` method: append new block or update existing block in-place, invalidate render cache
- [x] 2.3 Implement `FinalizeTaskList()` method: mark block as finalized and reset `taskListIdx` to `-1`
- [x] 2.4 Add `blockTaskList` render case to `doRender()` switch: render each task as `[indicator] content` row, apply lipgloss styling

## 3. Routing in model.go

- [x] 3.1 Add `lastToolWasTodo bool` field to `Model`
- [x] 3.2 In `MessageToolCall` case: when `ToolName == "TodoWrite"`, parse `ToolInput["todos"]` into `[]todoItem` and call `output.UpdateTaskList()`; set `lastToolWasTodo = true`; skip `AppendToolCall` and spinner
- [x] 3.3 In `MessageToolCall` case: reset `lastToolWasTodo = false` for all other tool names
- [x] 3.4 In `MessageToolResult` case: skip `AppendToolResult` when `lastToolWasTodo` is true; reset flag
- [x] 3.5 In `MessageDone` case: call `output.FinalizeTaskList()`

## 4. Styling

- [x] 4.1 Add lipgloss styles for task list to `Styles` struct: pending indicator, in-progress indicator, completed indicator, task content
- [x] 4.2 Wire new styles into the `NewStyles()` / theme initialization

## 5. Tests

- [x] 5.1 Unit test `UpdateTaskList`: verify new block appended on first call, block updated in-place on second call
- [x] 5.2 Unit test `FinalizeTaskList`: verify block marked finalized, `taskListIdx` reset to `-1`
- [x] 5.3 Unit test `doRender` for `blockTaskList`: verify each status renders the correct indicator prefix
- [x] 5.4 Unit test routing: `MessageToolCall` with `TodoWrite` does not produce a `blockToolCall` block; `MessageToolResult` after `TodoWrite` does not produce a `blockToolResult` block

## 6. Documentation

- [x] 6.1 Update README.md to document task list display behavior

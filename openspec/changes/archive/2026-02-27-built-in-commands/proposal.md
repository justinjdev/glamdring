## Why

Glamdring has no built-in commands. All slash commands come from user-defined `.claude/commands/` markdown files that expand into agent prompts. Users have no way to control the TUI itself — clear the screen, check costs, switch models, or get help — without quitting and restarting.

## What Changes

- Add a built-in command system that intercepts slash commands before they reach the user-defined command registry
- `/help` — list all available commands (built-in + user-defined)
- `/clear` — clear conversation output and reset token counters
- `/config` — display current settings (model, max turns, MCP servers)
- `/model <name>` — switch the active model for subsequent turns
- `/compact` — compact/summarize conversation context (user has specific requirements for this)
- `/cost` — display cumulative token usage and estimated cost
- `/quit` — exit glamdring

## Capabilities

### New Capabilities
- `built-in-commands`: Command dispatch system that handles built-in TUI commands before falling through to user-defined slash commands. Each command executes locally without invoking the agent.

### Modified Capabilities

## Impact

- `internal/tui/model.go` — intercept slash commands in `handleSubmit` before agent dispatch
- `internal/tui/output.go` — add `Clear()` method
- `internal/tui/commands.go` — extend with built-in command definitions and dispatch
- `internal/tui/statusbar.go` — may need reset support for `/clear`
- No new dependencies

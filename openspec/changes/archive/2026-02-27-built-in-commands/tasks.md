## 1. Command Dispatch Infrastructure

- [ ] 1.1 Add built-in command registry with name→handler map in `internal/tui/commands.go`
- [ ] 1.2 Add `AppendSystem` method to `OutputModel` for displaying command output (visually distinct from agent/user messages)
- [ ] 1.3 Intercept built-in commands in `handleSubmit` before user-defined command lookup
- [ ] 1.4 Merge built-in command names into `SlashCommandState` for tab completion

## 2. Simple Commands

- [ ] 2.1 Implement `/help` — list all built-in commands with descriptions, then user-defined commands
- [ ] 2.2 Implement `/quit` — return `tea.Quit`
- [ ] 2.3 Implement `/clear` — add `Clear()` method to `OutputModel`, reset token/turn counters in statusbar
- [ ] 2.4 Implement `/cost` — format and display cumulative input/output tokens, estimated cost, turn count
- [ ] 2.5 Implement `/config` — display current model, max turns, MCP server names
- [ ] 2.6 Implement `/model <name>` — update `agentCfg.Model`, refresh statusbar; show current model if no arg

## 3. Compact Command

- [ ] 3.1 Implement `/compact` — send structured summarization prompt to agent with instructions for aggressive compression
- [ ] 3.2 Write compact summary to `tmp/checkpoint.md` in working directory (create `tmp/` if needed) with timestamp and git branch header
- [ ] 3.3 After summary completes, truncate conversation history to system prompt + compact summary
- [ ] 3.4 Clear output viewport and display the compact summary as the new starting context

## 4. Tests

- [ ] 4.1 Test built-in command dispatch precedence over user-defined commands
- [ ] 4.2 Test `/model` with and without argument
- [ ] 4.3 Test `/clear` resets output and counters

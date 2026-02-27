## Context

Glamdring's slash command system currently only supports user-defined commands discovered from `.claude/commands/*.md` files. These expand into agent prompts — there's no mechanism for commands that control the TUI itself.

The TUI already has the infrastructure: `handleSubmit` checks `IsSlashCommand`, looks up in the registry, and expands. Built-in commands need to intercept before this lookup and execute locally without invoking the agent.

Key files: `internal/tui/model.go` (handleSubmit), `internal/tui/commands.go` (parsing), `internal/tui/output.go` (display), `internal/tui/statusbar.go` (token tracking).

## Goals / Non-Goals

**Goals:**
- Built-in commands that execute locally in the TUI (no agent invocation)
- `/help`, `/clear`, `/config`, `/model`, `/compact`, `/cost`, `/quit`
- Built-ins take precedence over user-defined commands with the same name
- `/compact` follows the lembas pattern: structured summary → checkpoint file → context compaction
- Tab completion includes built-in commands

**Non-Goals:**
- Persistent config editing via `/config` (read-only for now)
- Plugin/extension system for commands
- Keybinding customization for commands

## Decisions

### 1. Command dispatch order

**Decision**: In `handleSubmit`, check built-in commands first, then fall through to the user-defined command registry, then treat as agent prompt.

**Rationale**: Built-ins must always work — a user-defined `/help.md` shouldn't shadow the built-in `/help`. Simple precedence: built-in > user-defined > agent.

### 2. Built-in commands execute synchronously in the TUI

**Decision**: Built-in commands return a `tea.Cmd` (or nil) directly from `handleSubmit`. They do not enter `StateRunning` or invoke the agent.

**Alternative considered**: Running built-in commands through the agent. Rejected — these are TUI-level operations that don't need LLM involvement.

### 3. `/compact` uses structured summarization then triggers compaction

**Decision**: `/compact` works in two steps:
1. Send a summarization prompt to the agent that produces a structured context block (task, findings, files, state, next steps). The prompt instructs aggressive compression — discard raw output, keep only conclusions and decisions.
2. Write the summary to `tmp/checkpoint.md` in the repo root (with timestamp, phase, branch).
3. After the summary is displayed and saved, the conversation messages are truncated to just the system prompt + the compact summary, freeing context window.

**Rationale**: Matches the lembas pattern. The checkpoint file survives session crashes. The structured format ensures nothing critical is lost while noise is dropped.

### 4. `/model` mutates `agentCfg.Model` for subsequent turns

**Decision**: `/model <name>` updates `m.agentCfg.Model` and refreshes the statusbar. Takes effect on the next agent turn. No validation against a known model list — the API will reject invalid models.

**Rationale**: The model is already set per-turn via `agentCfg`. Mutating it is trivial. Avoiding a hardcoded model list means we don't need updates when new models ship.

### 5. `/clear` resets output and token counters

**Decision**: `/clear` clears the output viewport, resets the block list, and zeroes the token/turn counters in the statusbar. Does not affect the agent's conversation history (that's what `/compact` is for).

**Alternative considered**: Also clearing conversation history. Rejected — `/clear` is a visual reset; `/compact` is a context reset.

### 6. Output for built-in commands

**Decision**: Built-in command output is displayed as system messages in the output panel using a new `AppendSystem` method — visually distinct from user messages and agent responses.

### 7. Tab completion includes built-ins

**Decision**: Merge built-in command names into the `SlashCommandState` available commands list alongside user-defined commands.

## Risks / Trade-offs

- **`/compact` requires agent invocation** → Unlike other built-ins, `/compact` sends a prompt to the agent for summarization. This means it enters `StateRunning` briefly. Mitigation: it's a special case — the handler starts the agent with a compaction prompt, then truncates history when done.
- **No model validation** → `/model gpt-4` will be accepted locally but fail on the next API call. Mitigation: acceptable UX — the error is immediate and obvious.
- **Checkpoint file location** → `tmp/checkpoint.md` assumes the user runs glamdring from the repo root. Mitigation: use the CWD from config, ensure `tmp/` directory exists.

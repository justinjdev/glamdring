## Why

glamdring loses all conversation history on restart, `/clear`, or process exit — users cannot resume interrupted sessions or reference prior work. JSONL-based persistence is a low-complexity first step that delivers cross-session continuity without new dependencies or query infrastructure.

## What Changes

- Each conversation session is assigned a UUID and auto-saved to `~/.glamdring/sessions/<id>.jsonl` as messages accumulate
- On startup, glamdring offers to restore the most recent session (opt-in)
- A `/session` built-in command exposes list, resume, and delete operations
- Configuration adds `persistence.enabled` (default: true) and `persistence.dir` (optional override)
- Session files use newline-delimited JSON, one `api.RequestMessage` per line

## Capabilities

### New Capabilities

- `session-persistence`: Append-only JSONL storage for conversation history; session CRUD; startup restore; `/session` command

### Modified Capabilities

- `agent-loop`: Session ID assigned at loop start; messages appended to JSONL file after each turn
- `built-in-commands`: `/session` command added for list/resume/delete operations
- `config-loader`: New `persistence` config block loaded and validated at startup

## Impact

- `pkg/agent/session.go` — add session ID, JSONL writer, append-on-turn logic
- `pkg/agent/config.go` — add `MemoryConfig` (enabled, dir) passed into Session
- `pkg/session/` — new package: JSONL read/write, session index, restore logic
- `pkg/config/settings.go` — new `Persistence` settings block
- `internal/tui/model.go` — startup restore prompt; wire `/session` command
- `cmd/glamdring/main.go` — initialize session store from config
- No new external dependencies (uses `encoding/json`, `os`, `github.com/google/uuid` already indirect)

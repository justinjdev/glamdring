## 1. Session Package

- [x] 1.1 Create `pkg/session/` package with `Store` struct; implement `Open(dir string) (*Store, error)` that creates the sessions directory if absent
- [x] 1.2 Implement `Store.NewSession() (Session, error)` — generates UUID v4, initializes metadata with `created_at` and title "New Session", opens JSONL file for append
- [x] 1.3 Implement `Store.AppendMessages(id string, msgs []api.RequestMessage) error` — marshals each message to JSON and appends as a newline to `<dir>/<id>.jsonl`
- [x] 1.4 Implement `Store.LoadMessages(id string) ([]api.RequestMessage, error)` — reads JSONL file line by line, unmarshals each line into `api.RequestMessage`
- [x] 1.5 Implement `Store.CloseSession(id string, messageCount int) error` — finalizes session metadata (updated_at, message_count, title from first user message), rewrites index atomically via write-then-rename
- [x] 1.6 Implement `Store.ListSessions() ([]SessionMeta, error)` — reads and returns `index.json` sorted reverse-chronologically; returns empty slice if file absent
- [x] 1.7 Implement `Store.DeleteSession(id string) error` — removes `<id>.jsonl` and updates index atomically
- [x] 1.8 Implement `Store.RebuildIndex() (int, error)` — scans `*.jsonl` files, reads first user message from each for title, writes fresh `index.json`
- [x] 1.9 Add unit tests covering: append+load roundtrip, atomic index rewrite, missing file errors, rebuild from scan

## 2. Config

- [x] 2.1 Add `Persistence` struct to `pkg/config/settings.go` with fields `Enabled bool` (default true) and `Dir string` (default "")
- [x] 2.2 Add `Persistence Persistence` field to `Settings` struct and update `DefaultSettings()` to set `Enabled: true`
- [x] 2.3 Add helper `Settings.SessionsDir() string` — returns `Dir` if set, otherwise `~/.glamdring/sessions/`

## 3. Agent Integration

- [x] 3.1 Add `Store *session.Store` and `SessionID string` fields to `pkg/agent/session.go` `Session` struct
- [x] 3.2 Add `lastSavedIndex int` field to `Session` struct for tracking incremental save progress
- [x] 3.3 In `NewSession()`, if `cfg.Store` is non-nil, call `Store.NewSession()` and assign `SessionID`; log and disable store on error
- [x] 3.4 In `runTurn()`, after appending the assistant response, call `store.AppendMessages(sessionID, messages[lastSavedIndex:])` and advance `lastSavedIndex`; log errors without aborting
- [x] 3.5 Add `Store *session.Store` field to `agent.Config` in `pkg/agent/config.go`
- [x] 3.6 Add `Session.Close() error` method — calls `store.CloseSession(sessionID, len(messages))`; no-op if store is nil

## 4. TUI Wiring

- [x] 4.1 In `cmd/glamdring/main.go`, initialize `session.Store` from config when `persistence.enabled` is true; pass store into `agent.Config`
- [x] 4.2 In `internal/tui/model.go` startup, after model init: if store is non-nil and `ListSessions()` returns at least one entry, display restore prompt `Resume last session "<title>"? [y/N]`
- [x] 4.3 If user responds Y to restore prompt, call `store.LoadMessages(lastID)`, set as session messages, set `lastSavedIndex` to len(messages)
- [x] 4.4 On `/clear` command, call `session.Close()` then `store.NewSession()` to start a fresh session before resetting conversation

## 5. /session Command

- [x] 5.1 Register `/session` as a built-in command in the TUI command dispatch
- [x] 5.2 Implement `/session list` (default when no subcommand): read index, render table with truncated ID (8 chars), title, date, message count
- [x] 5.3 Implement `/session resume <id>`: call `store.LoadMessages`, replace session messages, update `lastSavedIndex`, display confirmation
- [x] 5.4 Implement `/session delete <id>`: call `store.DeleteSession`, display confirmation or error
- [x] 5.5 Implement `/session repair`: call `store.RebuildIndex`, display count of sessions found
- [x] 5.6 Handle unknown subcommands: display `Usage: /session [list|resume <id>|delete <id>|repair]`
- [x] 5.7 Handle persistence-disabled case for all subcommands: display "Session persistence is disabled"
- [x] 5.8 Add `/session` to `/help` output and tab completion

## 6. README

- [x] 6.1 Update README.md with session persistence feature: storage location, config options, `/session` command reference, and how to disable

## Context

glamdring's `Session` struct holds `messages []api.RequestMessage` in memory only. There is no persistence layer; all history is lost on exit. The codebase already uses `encoding/json` and `github.com/google/uuid` (indirect), so JSONL persistence adds no new external dependencies.

The existing `pkg/index/` package provides a precedent for a self-contained storage package. The `/compact` command writes checkpoint files to disk, establishing that glamdring does persist structured data — session files extend this naturally.

## Goals / Non-Goals

**Goals:**
- Persist each conversation session as an append-only JSONL file on disk
- Auto-assign a UUID to each session; write one `api.RequestMessage` per line after each turn
- Restore the previous session on startup (opt-in prompt in TUI)
- Provide `/session list`, `/session resume <id>`, `/session delete <id>` built-in subcommands
- Config block to enable/disable persistence and override storage directory

**Non-Goals:**
- Full-text or semantic search over session history (deferred; use SQLite/FTS5 in a future change if needed)
- Session sharing or export
- Automatic summarization or compaction of old sessions
- Team/subagent session persistence (out of scope for this change)

## Decisions

### Decision 1: One file per session, append-only

**Chosen:** `~/.glamdring/sessions/<uuid>.jsonl`, one `api.RequestMessage` JSON object per line, appended after each turn.

**Alternatives considered:**
- Single database file (SQLite) — more queryable but requires a DB driver and migrations; deferred until search is needed
- One file for all sessions — simpler path management but requires locking and random-access updates; append-only per-file is safer

**Rationale:** Append-only writes are lock-free and crash-safe. File-per-session makes restore, delete, and listing trivial. JSONL round-trips cleanly through `encoding/json` with no schema version concerns.

### Decision 2: Session index as a sidecar file

**Chosen:** `~/.glamdring/sessions/index.json` — a JSON array of `{id, title, created_at, updated_at, message_count}` entries, rewritten atomically on each session close.

**Alternatives considered:**
- Derive index by scanning `*.jsonl` files — correct but O(n×m) on startup; requires opening each file to read metadata
- Embed metadata in the first line of each JSONL file — complicates the append-only invariant and reader logic

**Rationale:** The index file makes `list` and startup-restore O(1) reads. Atomic rewrite (write to `.tmp`, rename) prevents corruption. The index is a cache — it can be rebuilt by scanning session files if corrupted.

### Decision 3: Opt-in restore prompt on startup

**Chosen:** On startup, if a previous session exists and `persistence.enabled` is true, the TUI shows a one-line prompt: `Resume last session? [y/N]`. Declined sessions are not re-prompted.

**Alternatives considered:**
- Auto-restore unconditionally — surprising for users who want a fresh start
- Never auto-restore; require explicit `/session resume` — loses discoverability

**Rationale:** Opt-in with a Y/N prompt is the minimal discoverable UX. The default is N (fresh session), consistent with current behavior.

### Decision 4: New `pkg/session/` package, thin integration into `pkg/agent/`

**Chosen:** `pkg/session/` owns all JSONL read/write and index management. `pkg/agent/Session` gains an optional `*session.Store` field; if nil, behavior is unchanged (no persistence).

**Alternatives considered:**
- Inline persistence directly into `pkg/agent/session.go` — mixes concerns; harder to test
- Persist from the TUI layer — agent is unaware of persistence, which creates a mismatch if agent is used headlessly

**Rationale:** Clean package boundary; `pkg/agent` stays testable without a real filesystem. Store is nil by default so existing tests require zero changes.

### Decision 5: Append after each assistant turn, not on every message

**Chosen:** After `runTurn()` completes and the assistant response is appended to `session.messages`, persist only the new messages since the last save (`lastSavedIndex`).

**Rationale:** Appending per-turn batches user + assistant messages together naturally. Single-message appends during a turn (e.g., tool results) would produce partial state that is harder to restore correctly.

## Risks / Trade-offs

- **Index drift** — if glamdring crashes before closing the index, the JSONL file exists but is absent from the index. Mitigation: `pkg/session` exposes a `Rebuild()` function that scans `*.jsonl` files and reconstructs the index; surfaced via `/session repair`.
- **Large sessions** — a very long session produces a large JSONL file. No size limit is enforced in this change. Mitigation: deferred to a future compaction feature; users can `/compact` and the summary is stored separately.
- **Concurrent instances** — two glamdring processes writing to the same session ID will corrupt the JSONL file. Mitigation: each process generates a unique UUID at startup, so files never collide. Index rewrite uses atomic rename to prevent partial reads.
- **Message content size** — `api.RequestMessage.Content` can be `string` or `[]ContentBlock` (image data, large tool outputs). JSONL files may grow large. Accepted trade-off for this phase.

## Migration Plan

- No migration required. Existing installations have no session files; the feature activates silently when `persistence.enabled` is true (default).
- If `~/.glamdring/sessions/` does not exist, `pkg/session` creates it on first write.
- Disabling persistence (`persistence.enabled: false`) leaves existing files untouched.

## Open Questions

- Should the title shown in `/session list` be the first N characters of the first user message, or should it be explicitly settable? (Lean toward auto-derived for now.)
- Should `/clear` start a new session file, or continue appending to the current one? (Lean toward new session per `/clear` to match user expectation of a fresh start.)

## Context

Glamdring's agent currently understands codebases through brute-force tools (Glob, Grep, Read). Shire builds a structured index — packages, dependencies, symbols, files — in SQLite with FTS5, and serves it over MCP. Rather than porting shire's full indexing pipeline to Go, this change adds Go bindings that read shire's existing `.shire/index.db` directly and expose the queries as built-in glamdring tools. Shire remains the indexer; glamdring becomes a native consumer.

## Goals / Non-Goals

**Goals:**
- Read-only Go query layer for shire's SQLite schema
- 13 built-in tools matching shire's MCP tool surface
- Automatic detection of `.shire/index.db` at startup
- Shell out to `shire build` for rebuilds via `/index rebuild`
- Zero configuration — works if the index exists, invisible if it doesn't

**Non-Goals:**
- Reimplementing manifest parsing, symbol extraction, or the build pipeline in Go
- Watch daemon or automatic re-indexing
- MCP server mode (shire already handles that)
- Modifying shire's database schema

## Decisions

### 1. Go bindings live in glamdring (pkg/index/)

**Choice:** The query layer and tool implementations live in `pkg/index/` within glamdring.

**Alternatives considered:**
- Separate `shire-go` library repo — adds a dependency to manage, overkill for a read-only query layer
- Inside the shire repo as a `go/` subdirectory — mixes Rust and Go build systems, awkward for contributors

**Rationale:** The bindings are tightly coupled to glamdring's Tool interface. The query functions are straightforward SQL — ~300 lines total. No reason to extract a separate library until another Go consumer appears.

### 2. Pure-Go SQLite via modernc.org/sqlite

**Choice:** `modernc.org/sqlite` (transpiled C SQLite to Go, no CGO)

**Alternatives considered:**
- `mattn/go-sqlite3` — requires CGO and a C compiler, complicates cross-compilation
- Reading shire's index via MCP — adds IPC overhead and MCP server management

**Rationale:** Zero CGO means glamdring stays a single static binary. Read-only WAL mode access is well-supported. FTS5 queries work identically to shire's Rust side.

### 3. Conditional tool registration

**Choice:** At startup, check for `.shire/index.db`. If present, open it read-only and register all 13 tools. If absent, skip silently.

**Alternatives considered:**
- Always register tools, return errors if no index — clutters the agent's tool list when no index exists
- Require explicit opt-in via config — adds friction for the common case

**Rationale:** Auto-detection is the simplest UX. The agent sees index tools when they're useful, doesn't see them when they're not. No config needed.

### 4. Shell out to shire for rebuilds

**Choice:** `/index rebuild` runs `shire build --root <cwd>` as a subprocess, then reopens the database.

**Alternatives considered:**
- Embed shire as a library (FFI) — complex, fragile, unnecessary
- Don't support rebuilds — forces users to leave glamdring to run shire manually

**Rationale:** Shelling out is simple and reliable. The shire binary is already on PATH for users who have an index. Graceful error if shire isn't installed.

## Risks / Trade-offs

**[Schema coupling]** Glamdring's queries must match shire's SQLite schema. If shire changes its schema, glamdring's bindings break.
→ **Mitigation:** Shire's schema is stable and versioned via `shire_meta` table. Add a schema version check on database open — warn if unexpected version. Both repos are owned by the same developer.

**[shire not installed]** Users without shire can't create or rebuild indexes.
→ **Mitigation:** `/index rebuild` shows install instructions if shire isn't on PATH. Index tools simply don't appear if no database exists. Zero degradation of core glamdring functionality.

**[Stale index]** The index may be out of date if shire hasn't been run recently.
→ **Mitigation:** `index_status` tool shows when the index was last built. The agent can check freshness. Users can run `/index rebuild` or set up shire's watch daemon externally.

## Open Questions

1. **Auto-rebuild after file mutations:** Should glamdring shell out to `shire rebuild --stdin` after Edit/Write/Bash tool calls (mirroring shire's PostToolUse hook pattern)? Trade-off is subprocess overhead vs index freshness.
2. **Index tools in system prompt:** Should the system prompt mention available index tools to guide the agent, or rely on tool descriptions alone?

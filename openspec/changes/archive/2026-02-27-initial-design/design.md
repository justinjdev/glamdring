## Context

Claude Code is a Node.js/Ink-based agentic coding TUI. Its rendering layer (Ink = React for terminals) causes persistent issues: flickering, layout glitches, high memory usage, slow startup, and poor input handling. The Agent SDK is not a separable engine — it wraps the full Claude Code process. There is no way to build a better frontend without reimplementing the backend.

Glamdring is a from-scratch reimplementation in Go. It talks directly to the Claude Messages API and implements its own tool execution, permission model, and TUI. The architecture separates the agent engine (`pkg/`) from the terminal frontend (`internal/tui/`) so the engine can later be consumed by other frontends or exposed as a daemon.

## Goals / Non-Goals

**Goals:**
- Feature parity with Claude Code's core workflow: agentic loop, built-in tools, MCP, CLAUDE.md, hooks, commands, custom agents, subagents
- Fast startup, low memory footprint, responsive UI
- Clean library API in `pkg/` that doesn't depend on the TUI
- Compatibility with existing `.claude/` configuration (CLAUDE.md, commands, agents, MCP server configs)
- Polished terminal experience with streaming markdown, syntax highlighting, and clear permission prompts

**Non-Goals:**
- Daemon/IPC mode (design for it, don't build it yet)
- Web UI or editor plugin frontends (future consumers of `pkg/`)
- Feature parity with every Claude Code feature — focus on what's actually used
- Backwards compatibility with the Agent SDK's `query()` API — this is a new tool, not a drop-in replacement
- Cross-platform portability concerns — macOS is the target

## Decisions

### Language: Go
**Rationale:** Fast compilation, instant startup, minimal memory, excellent stdlib (HTTP client, JSON, file I/O, process management). Goroutines map naturally to "stream API response while handling user input." Simpler ownership model than Rust for UI code where state is passed around frequently. The performance bottleneck is API latency, not local computation — Go's speed is more than sufficient.

**Alternatives considered:**
- Rust + ratatui: Better raw performance and compile-time guarantees, but ownership friction in UI code adds development time without meaningful runtime benefit for this use case.
- TypeScript (no Ink): Lowest effort but keeps the Node.js runtime and its overhead, defeating the purpose.

### TUI: Charm stack (bubbletea + lipgloss + glamour + bubbles)
**Rationale:** Bubbletea's Elm architecture (Model → Update → View) enforces clean state management. Lipgloss handles styling without escape code soup. Glamour renders markdown with syntax highlighting out of the box. Bubbles provides battle-tested input and viewport components. The entire Charm ecosystem is designed to work together.

**Alternatives considered:**
- Raw ANSI + readline: Maximum control but reimplements solved problems (scrolling, reflow, styling).
- blessed/tcell: Lower-level, more boilerplate, less Go-idiomatic.

### Architecture: Library-first monolith
**Rationale:** `pkg/` exposes a channel-based API (`agent.Run() → <-chan Message`). The TUI in `internal/tui/` consumes this channel. Everything runs in one process. The package boundary is the seam — if a daemon mode is needed later, wrap `pkg/agent` in a thin server. No IPC protocol design or serialization overhead until there's a real consumer.

**Alternatives considered:**
- Split process from day one: Adds IPC protocol, process lifecycle management, reconnection logic before we have a working agent. Over-engineering.
- No library separation: Works initially but makes extraction painful later.

### API integration: Direct HTTP to Messages API
**Rationale:** The Claude Messages API is a single endpoint (`POST /v1/messages`). SSE streaming is well-specified. Go's `net/http` + `encoding/json` handle this without external dependencies. No need for an SDK when the API surface is this simple.

### Tool interface: Go interface with JSON schema
```go
type Tool interface {
    Name() string
    Description() string
    Schema() json.RawMessage
    Execute(ctx context.Context, input json.RawMessage) (ToolResult, error)
}
```
**Rationale:** MCP tools and built-in tools implement the same interface. The registry dispatches by name. JSON schema is passed directly to the API. Adding a new tool means implementing one interface — no code generation, no registration boilerplate beyond a single `Register()` call.

### Permission model: Three tiers with callback
**Rationale:** Read-only tools (Read, Glob, Grep) are always allowed. Side-effect tools (Write, Edit, Bash) prompt the user. The agent sends a `PermissionRequest` message on its output channel; the consumer (TUI or future daemon) handles the prompt and responds via a callback channel on the message. This keeps the agent decoupled from any specific UI.

### MCP: Stdio transport first, SSE later
**Rationale:** Most MCP servers use stdio (spawn process, JSON-RPC over stdin/stdout). SSE transport is less common. Build stdio first, add SSE when needed. Existing Go MCP libraries can be evaluated, but the protocol is simple enough to implement directly if they're immature.

### Configuration compatibility
**Rationale:** Read the same `.claude/CLAUDE.md`, `.claude/commands/`, `.claude/agents/` paths and formats as Claude Code. Users shouldn't need to reconfigure anything to switch tools. MCP server configuration format should also be compatible.

## Risks / Trade-offs

**System prompt fidelity** — Claude Code's effectiveness comes partly from its system prompt engineering. We need to study the Claude Code source to understand how it assembles tool descriptions, instructions, and context into the system prompt. Getting this wrong means worse agent behavior.
→ Mitigation: Read the Claude Code TypeScript source. Start with a minimal system prompt and iterate based on agent behavior.

**MCP ecosystem maturity in Go** — Go's MCP client libraries are newer and less battle-tested than the Node.js equivalents.
→ Mitigation: The MCP protocol (JSON-RPC over stdio) is simple. We can implement the subset we need directly if existing libraries are insufficient.

**Edit tool complexity** — Claude Code's Edit tool does precise string replacement with conflict detection. Reimplementing this correctly (handling whitespace, partial matches, uniqueness checks) is fiddly.
→ Mitigation: This is a well-defined problem. Port the logic carefully, test extensively.

**Subagent isolation** — Spawning parallel agent goroutines that share a filesystem requires careful coordination to avoid conflicts.
→ Mitigation: Each subagent gets its own conversation context. File-level conflicts are the user's responsibility (same as Claude Code).

**Keeping up with Claude API changes** — New API features (compaction, new tool types) require manual integration.
→ Mitigation: The API is stable and versioned. New features are additive. We can adopt them incrementally.

## Open Questions

- **Session persistence format:** Should we use the same format as Claude Code for session files, or design our own? Compatibility would allow resuming Claude Code sessions in glamdring, but may constrain our data model.
- **Config file for glamdring-specific settings:** Use a dedicated `glamdring.toml`/`glamdring.yaml`, or extend the `.claude/settings.json` format?
- **Streaming markdown rendering:** Glamour renders complete markdown strings. For streaming, we need to handle partial markdown (incomplete code blocks, mid-list rendering). Investigate whether glamour supports incremental rendering or if we need a custom approach.

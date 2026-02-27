## Why

Claude Code's Ink-based TUI suffers from rendering bugs, input handling issues, sluggish feel, and memory/CPU bloat. The Agent SDK is a wrapper around Claude Code's engine — there's no lightweight protocol to build on top of. We need a ground-up reimplementation of the agentic coding loop as a fast, native Go binary with a polished terminal UI, designed as a reusable library that can later support alternative frontends (web, editor plugins, daemon mode).

## What Changes

- New standalone CLI tool (`glamdring`) that replaces Claude Code for agentic coding workflows
- Direct integration with Claude Messages API — no dependency on Node.js or the Agent SDK
- Full agentic loop: streaming responses, tool execution, permission prompts, multi-turn conversation
- Built-in tool implementations in Go: Read, Write, Edit, Bash, Glob, Grep
- MCP client supporting stdio and SSE transports for external tool servers
- CLAUDE.md discovery and loading (project-level and user-level)
- Hooks system (PreToolUse, PostToolUse, SessionStart, etc.)
- Custom slash commands from `.claude/commands/` directories
- Custom agent definitions from `.claude/agents/`
- Subagent spawning for parallel task execution
- TUI built on Charm stack (bubbletea, lip gloss, glamour) with streaming markdown, syntax highlighting, and permission prompts

## Capabilities

### New Capabilities
- `api-client`: HTTP client for Claude Messages API with SSE streaming, adaptive thinking, compaction support
- `agent-loop`: Core agentic loop — send message, handle tool_use responses, execute tools, feed results back, repeat until end_turn
- `tool-system`: Tool interface, registry, dispatch, and built-in implementations (Read, Write, Edit, Bash, Glob, Grep)
- `permission-system`: Three-tier permission model (always-allow, prompt-user, always-block) with configurable rules
- `mcp-client`: MCP client with stdio and SSE transports, tool discovery, unified tool dispatch
- `config-loader`: CLAUDE.md discovery (walk up from CWD), settings resolution, system prompt assembly
- `hooks`: Event hook system — shell commands triggered by agent lifecycle events
- `commands`: Slash command discovery from `.claude/commands/` directories, prompt template expansion
- `custom-agents`: Agent definitions from `.claude/agents/`, spawnable via subagent system
- `subagents`: Parallel agent task spawning with isolated contexts
- `tui`: Bubbletea-based terminal UI — input pane, streaming output with markdown rendering, permission prompts, status bar

### Modified Capabilities

(none — greenfield project)

## Impact

- **Dependencies:** Go stdlib + Charm stack (bubbletea, lipgloss, glamour, bubbles). No Node.js runtime required.
- **Architecture:** Library-first design. `pkg/` contains the reusable engine (agent, tools, mcp, config). `internal/tui/` contains the terminal frontend. `cmd/glamdring/` is the entry point. Clean package boundaries enable future process split (daemon mode) without refactoring.
- **Compatibility:** Reads the same `.claude/CLAUDE.md`, `.claude/commands/`, `.claude/agents/`, and MCP server configs as Claude Code. Users can switch between tools without reconfiguring.
- **API:** Talks directly to `POST /v1/messages` with `ANTHROPIC_API_KEY`. No intermediate SDK or proxy.

# glamdring

A fast, native TUI for agentic coding with Claude. Built in Go with [Charm](https://charm.sh) libraries, replacing Claude Code's Ink-based frontend with a lightweight, responsive alternative.

## Features

- **Agentic loop** — streaming responses, multi-turn conversations with persistent session memory, extended thinking, prompt caching support
- **Built-in tools** — Read, Write, Edit, Bash, Glob, Grep + [shire](https://github.com/justinjdev/shire) index tools (auto-detected, auto-rebuilt after file changes)
- **Permission system** — three-tier model (always-allow, prompt, block) with session-level overrides
- **MCP support** — connect external tool servers via stdio transport
- **CLAUDE.md** — discovers and loads project/user instructions automatically (bare `CLAUDE.md`, `.claude/CLAUDE.md`, and `.claude/CLAUDE.local.md` at every directory level)
- **Hooks** — shell commands triggered by agent lifecycle events (SessionStart on launch, SessionEnd on exit)
- **Checkpoint resume** — detects `tmp/checkpoint.md` from `/compact` and offers to load previous session context
- **Slash commands** — custom prompts from `.claude/commands/` with tab completion
- **Custom agents** — define specialized subagents in `.claude/agents/`
- **Subagents** — parallel task spawning via the Task tool

## Install

```
go install github.com/justin/glamdring/cmd/glamdring@latest
```

Or build from source:

```
git clone <repo-url>
cd glamdring
go build -o glamdring ./cmd/glamdring
```

## Usage

```
export ANTHROPIC_API_KEY=sk-ant-...
glamdring
```

### Flags

| Flag | Description |
|---|---|
| `--cwd <path>` | Set working directory (defaults to current) |
| `--model <id>` | Override model (default: `claude-opus-4-6`) |

### Keybindings

| Key | Action |
|---|---|
| `Enter` | Submit prompt |
| `Alt+Enter` | Insert newline |
| `j` / `k` | Scroll line up/down |
| `Ctrl+u` / `Ctrl+d` | Scroll half page |
| `G` / `g` | Jump to bottom/top |
| `y` / `n` / `a` | Permission: yes / no / always |
| `y` / `n` | Checkpoint prompt: load / skip |
| `Tab` | Complete slash command |
| `Ctrl+c` | Quit |

## Configuration

Glamdring reads the same configuration as Claude Code:

- **CLAUDE.md** — `~/.claude/CLAUDE.md` (user), `CLAUDE.md` and `.claude/CLAUDE.md` (project, all levels concatenated), `.claude/CLAUDE.local.md` (local overrides, gitignored)
- **Settings** — `~/.claude/settings.json` and `.claude/settings.json` (`max_turns` supports `0` for explicitly unlimited)
- **Commands** — `.claude/commands/*.md`
- **Agents** — `.claude/agents/*.md` or `.claude/agents/*.yaml`
- **Hooks** — `hooks` array in `settings.json`
- **Indexer** — `indexer` object in `settings.json`

### Indexer Configuration

The shire code indexer is auto-detected by default. Configure via `settings.json`:

```json
{
  "indexer": {
    "enabled": true,
    "command": "shire",
    "auto_rebuild": true
  }
}
```

| Field | Default | Description |
|---|---|---|
| `enabled` | auto-detect | `true` = force on, `false` = disable, omit = auto-detect `.shire/index.db` |
| `command` | `"shire"` | Binary name for the indexer |
| `auto_rebuild` | `true` | Rebuild index after agent turns that modify files |

## Architecture

```
pkg/
  agent/       Core agentic loop, Session (multi-turn memory), permission system
  api/         Claude Messages API client (HTTP + SSE, prompt caching, retry)
  tools/       Built-in tools + Task tool for subagents
  index/       Shire index Go bindings (read-only SQLite queries)
  mcp/         MCP client (stdio JSON-RPC)
  config/      CLAUDE.md discovery, system prompt, settings
  hooks/       Event hook system
  commands/    Slash command discovery + expansion
  agents/      Custom agent definitions

internal/
  tui/         Bubbletea TUI (not part of library API)

cmd/
  glamdring/   Entry point
```

`pkg/` is the reusable engine. `internal/tui/` is the terminal frontend. The package boundary is designed so a daemon mode or alternative frontends can consume `pkg/` directly.

## License

Apache License 2.0. See [LICENSE](LICENSE).

# glamdring

A fast, native TUI for agentic coding with Claude. Built in Go with [Charm](https://charm.sh) libraries, replacing Claude Code's Ink-based frontend with a lightweight, responsive alternative.

## Features

- **Agentic loop** — streaming responses, multi-turn conversations, adaptive thinking
- **Built-in tools** — Read, Write, Edit, Bash, Glob, Grep
- **Permission system** — three-tier model (always-allow, prompt, block) with session-level overrides
- **MCP support** — connect external tool servers via stdio transport
- **CLAUDE.md** — discovers and loads project/user instructions automatically
- **Hooks** — shell commands triggered by agent lifecycle events (SessionStart hooks fire on launch)
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

- **CLAUDE.md** — `~/.claude/CLAUDE.md` (user) and `.claude/CLAUDE.md` (project)
- **Settings** — `~/.claude/settings.json` and `.claude/settings.json`
- **Commands** — `.claude/commands/*.md`
- **Agents** — `.claude/agents/*.md` or `.claude/agents/*.yaml`
- **Hooks** — `hooks` array in `settings.json`

## Architecture

```
pkg/
  agent/       Core agentic loop + permission system
  api/         Claude Messages API client (HTTP + SSE)
  tools/       Built-in tools + Task tool for subagents
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

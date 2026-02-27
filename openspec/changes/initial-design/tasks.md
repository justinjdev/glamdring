## 1. Project Scaffolding

- [ ] 1.1 Initialize Go module (`go mod init`), create directory structure (`cmd/glamdring/`, `pkg/`, `internal/tui/`)
- [ ] 1.2 Add Charm stack dependencies (bubbletea, lipgloss, glamour, bubbles)
- [ ] 1.3 Create `cmd/glamdring/main.go` entry point with CLI flag parsing (--cwd, --model)
- [ ] 1.4 Create core type definitions: `pkg/agent/message.go` (Message types, MessageType enum)

## 2. API Client

- [ ] 2.1 Implement HTTP client for `POST /v1/messages` with API key auth (`pkg/api/client.go`)
- [ ] 2.2 Implement SSE stream parser — parse `message_start`, `content_block_start`, `content_block_delta`, `content_block_stop`, `message_delta`, `message_stop` events
- [ ] 2.3 Implement request/response types including tool_use, tool_result, thinking blocks (`pkg/api/types.go`)
- [ ] 2.4 Implement retry logic with exponential backoff + jitter for 429/500/529 errors
- [ ] 2.5 Implement multi-turn conversation history management (append full response.content, maintain alternating roles)

## 3. Tool System

- [ ] 3.1 Define `Tool` interface and implement tool registry with dispatch-by-name (`pkg/tools/registry.go`)
- [ ] 3.2 Implement Read tool — file reading with line numbers, offset/limit support
- [ ] 3.3 Implement Write tool — file creation/overwrite with parent directory creation
- [ ] 3.4 Implement Edit tool — exact string replacement with uniqueness check and replace_all
- [ ] 3.5 Implement Bash tool — command execution with stdout/stderr capture, timeout, and SIGTERM on cancel
- [ ] 3.6 Implement Glob tool — file pattern matching with doublestar support, sorted by modification time
- [ ] 3.7 Implement Grep tool — regex search with output modes (content, files_with_matches, count), context lines, case-insensitive flag

## 4. Agent Loop

- [ ] 4.1 Implement core `agent.Run(ctx, Config) <-chan Message` function — send message, inspect stop_reason, dispatch tools, loop
- [ ] 4.2 Implement tool dispatch — match tool_use blocks to registered tools, execute, format tool_result
- [ ] 4.3 Handle multiple tool calls in a single response — execute all, return all results in one user message
- [ ] 4.4 Implement context cancellation — cancel in-flight HTTP requests and running subprocesses on ctx.Done()
- [ ] 4.5 Implement max turns limit with `MaxTurnsReached` message

## 5. Permission System

- [ ] 5.1 Implement three-tier classification — always-allow, prompt-user, always-block — with configurable tier assignment
- [ ] 5.2 Implement `PermissionRequest` message with tool name, input params, and human-readable summary
- [ ] 5.3 Implement permission response handling — approve, always-approve (session-level), deny
- [ ] 5.4 Wire permission checks into the agent loop's tool dispatch — block execution until response received

## 6. Config Loader

- [ ] 6.1 Implement CLAUDE.md discovery — walk up from CWD for project-level, check `~/.claude/` for user-level
- [ ] 6.2 Implement system prompt assembly — base instructions + tool descriptions + CLAUDE.md content
- [ ] 6.3 Implement `--cwd` flag handling and working directory resolution

## 7. TUI — Core

- [ ] 7.1 Create root bubbletea model (`internal/tui/model.go`) with app state, Init/Update/View cycle
- [ ] 7.2 Implement text input component — multiline editing, Enter to submit, Shift+Enter for newline
- [ ] 7.3 Implement streaming output viewport — receive TextDelta messages, render incrementally, auto-scroll
- [ ] 7.4 Implement markdown rendering via glamour with syntax-highlighted code blocks
- [ ] 7.5 Implement status bar — model name, token count, cost estimate, turn number

## 8. TUI — Interactions

- [ ] 8.1 Implement permission prompt UI — inline modal with y/n/a input, displays tool name and input summary
- [ ] 8.2 Implement tool call display — show tool name + summary inline, collapsible results for large output
- [ ] 8.3 Implement thinking block display — visually distinct style (dimmed/italic), streaming
- [ ] 8.4 Implement scrollable viewport — scroll up through history, preserve position when new content arrives
- [ ] 8.5 Implement slash command detection and tab completion in the input component

## 9. MCP Client

- [ ] 9.1 Implement MCP stdio transport — spawn process, JSON-RPC over stdin/stdout (`pkg/mcp/client.go`)
- [ ] 9.2 Implement MCP protocol — `initialize`, `tools/list`, `tools/call` requests and response parsing
- [ ] 9.3 Implement MCP tool adapter — wrap MCP tools in the `Tool` interface for unified dispatch
- [ ] 9.4 Implement MCP server lifecycle — start on agent init, handle crashes, clean shutdown on exit
- [ ] 9.5 Load MCP server configuration from settings

## 10. Hooks

- [ ] 10.1 Implement hook event system — define event types (PreToolUse, PostToolUse, SessionStart, SessionEnd, Stop)
- [ ] 10.2 Implement hook matching — regex matcher on tool name, event-based dispatch
- [ ] 10.3 Implement hook execution — run shell commands with tool context as env vars/stdin
- [ ] 10.4 Implement hook failure handling — PreToolUse blocks execution on failure, PostToolUse warns only
- [ ] 10.5 Load hook configuration from settings files

## 11. Commands & Custom Agents

- [ ] 11.1 Implement slash command discovery — scan `.claude/commands/` at project and user level
- [ ] 11.2 Implement command expansion — read markdown, replace `$ARGUMENTS`, send as prompt
- [ ] 11.3 Implement custom agent loader — scan `.claude/agents/`, parse definitions (name, description, prompt, tools)
- [ ] 11.4 Wire custom agents into subagent spawning — Task tool resolves subagent_type against custom agent definitions

## 12. Subagents

- [ ] 12.1 Implement Task tool — accepts prompt, subagent_type, tool restrictions; spawns a new agent goroutine
- [ ] 12.2 Implement subagent isolation — separate conversation history, shared filesystem, inherited permission model
- [ ] 12.3 Implement concurrent subagent execution — multiple goroutines, results returned via channels as they complete

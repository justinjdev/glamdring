# Glamdring Improvement Plan

Grouped into 10 implementation chunks, ordered by dependency and impact. Each chunk is self-contained and shippable independently. Chunks 1-7 are complete.

---

## Chunk 1: Critical Agent Loop Fixes [DONE] COMPLETED

**Priority:** P0 -- Nothing works correctly without these.
**Estimated scope:** ~400 lines changed across 4 files.
**Dependencies:** None (foundational).
**Status:** Completed. Branch `worktree-chunk1-agent-loop-fixes`.

### 1.1 Multi-turn conversation history

**Problem:** Each user message starts a fresh `agent.Run()` with no prior context. The agent has no memory of previous turns.

**Fix:** Move conversation history management out of `agent.Run` and into a persistent `Session` struct. The TUI holds the session; each user message appends to the history and calls a new turn method rather than spawning an entirely new agent loop.

**Files:**
- `pkg/agent/agent.go` -- Extract `messages []api.RequestMessage` from the `run()` closure into a `Session` struct. Add `Session.Turn(ctx, prompt string) <-chan Message` that appends the user message and runs one agentic loop iteration (which may involve multiple API calls for tool use).
- `pkg/agent/session.go` (new) -- `Session` struct holding conversation history, cumulative token usage, session-level permission overrides.
- `internal/tui/model.go` -- Hold a `*agent.Session` instead of recreating `agent.Config` per submit. `handleSubmit` calls `session.Turn()`.
- `cmd/glamdring/main.go` -- Create the session at startup, pass to TUI.

**Session struct sketch:**
```go
type Session struct {
    cfg      Config
    client   *api.Client
    registry *tools.Registry
    messages []api.RequestMessage
    sessionAllow map[string]bool
    totalInput   int
    totalOutput  int
    mu           sync.Mutex
}

func NewSession(cfg Config) *Session
func (s *Session) Turn(ctx context.Context, prompt string) <-chan Message
func (s *Session) Messages() []api.RequestMessage  // for /compact
func (s *Session) SetMessages(msgs []api.RequestMessage)  // after compaction
func (s *Session) Usage() (input, output int)
```

### 1.2 Fix thinking configuration

**Problem:** `ThinkingConfig` is missing `budget_tokens` (API rejects the request). `ContentBlock` is missing `signature` (multi-turn thinking fails).

**Files:**
- `pkg/api/types.go`:
  - Add `BudgetTokens int` to `ThinkingConfig` with `json:"budget_tokens,omitempty"`.
  - Add `Signature string` to `ContentBlock` with `json:"signature,omitempty"`.
- `pkg/api/client.go`:
  - Set `BudgetTokens: 10240` in the auto-config (line 59). Make this configurable via `Config`.
  - Increase `MaxTokens` default for thinking models (16384 is too low; use 32768 or make configurable).

### 1.3 Handle all stop reasons

**Problem:** `max_tokens` stop reason causes the loop to fall through and make an invalid API call.

**File:** `pkg/agent/agent.go`

**Fix:** Replace the if/else chain with a switch:
```go
switch turnResult.stopReason {
case "end_turn", "refusal":
    emit done, return
case "tool_use":
    execute tools, continue loop
case "max_tokens":
    // Append a user message asking the model to continue
    messages = append(messages, api.RequestMessage{
        Role:    "user",
        Content: "Your response was truncated. Please continue from where you left off.",
    })
    continue
default:
    // Unknown stop reason -- treat as done to avoid infinite loops
    emit done, return
}
```

### 1.4 Fix tool name casing

**Problem:** `BashTool.Name()` returns `"bash"` but the rest of the codebase expects `"Bash"`.

**File:** `pkg/tools/bash.go` -- Change `Name()` return to `"Bash"`.

Verify all tool `Name()` methods use PascalCase to match `alwaysAllowTools`, `permissionSummary`, and `turnModifiedFiles` checks. Audit:
- `ReadTool.Name()` -> `"Read"`
- `WriteTool.Name()` -> `"Write"`
- `EditTool.Name()` -> `"Edit"`
- `BashTool.Name()` -> `"Bash"` (currently `"bash"` -- fix)
- `GlobTool.Name()` -> `"Glob"`
- `GrepTool.Name()` -> `"Grep"`
- `TaskTool.Name()` -> `"Task"`

---

## Chunk 2: API Client Hardening [DONE] COMPLETED

**Priority:** P0 -- Goroutine leaks and stream failures.
**Estimated scope:** ~100 lines changed across 3 files.
**Dependencies:** None.
**Status:** Completed. PR #2.

### 2.1 Fix goroutine leak and double-close in Stream()

**Problem:** The context-cancellation goroutine in `Stream()` leaks on normal completion and races with the forwarding goroutine on `resp.Body.Close()`.

**File:** `pkg/api/client.go`

**Fix:** Remove the standalone context-cancellation goroutine entirely. Use a single goroutine with `sync.Once` for body close:

```go
func (c *Client) Stream(ctx context.Context, req *MessageRequest) (<-chan StreamEvent, error) {
    // ... marshal, doWithRetry ...

    var closeOnce sync.Once
    closeBody := func() { closeOnce.Do(func() { resp.Body.Close() }) }

    done := make(chan struct{})
    events := parseSSE(resp.Body, done)

    out := make(chan StreamEvent, 16)
    go func() {
        defer close(out)
        defer closeBody()
        for ev := range events {
            select {
            case out <- ev:
            case <-ctx.Done():
                close(done)
                closeBody() // unblock scanner
                return
            }
        }
    }()

    return out, nil
}
```

### 2.2 Increase SSE scanner buffer

**File:** `pkg/api/sse.go`

**Fix:** After `scanner := bufio.NewScanner(r)`, add:
```go
scanner.Buffer(make([]byte, 0, 1<<20), 10<<20) // 1MB initial, 10MB max
```

### 2.3 Improve APIError.Error()

**File:** `pkg/api/types.go`

**Fix:**
```go
func (e *APIError) Error() string {
    return fmt.Sprintf("api error %d (%s): %s", e.StatusCode, e.Type, e.Message)
}
```

### 2.4 Remove dead code

**File:** `pkg/api/retry.go` -- Remove `nonRetryableStatusCodes` (defined but never used).

### 2.5 Add system prompt array support

**File:** `pkg/api/types.go`

**Fix:** Change `System string` to `System any` in `MessageRequest`. This allows both string (simple) and `[]SystemBlock` (with cache control) formats. Add:

```go
type SystemBlock struct {
    Type         string        `json:"type"`
    Text         string        `json:"text,omitempty"`
    CacheControl *CacheControl `json:"cache_control,omitempty"`
}

type CacheControl struct {
    Type string `json:"type"` // "ephemeral"
}
```

### 2.6 Add cache usage tracking

**File:** `pkg/api/types.go`

**Fix:** Extend `Usage`:
```go
type Usage struct {
    InputTokens              int `json:"input_tokens"`
    OutputTokens             int `json:"output_tokens"`
    CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
    CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}
```

Propagate through `agent.Message` and `statusbar.go` for display.

---

## Chunk 3: Tool Completeness [DONE] COMPLETED

**Priority:** P1 -- Model will try to use features that don't exist.
**Estimated scope:** ~350 lines changed across 7 files.
**Dependencies:** None.
**Status:** Completed. PR #6.

### 3.1 Bash improvements

**File:** `pkg/tools/bash.go`

- **Fix timeout detection:** Check `ctx.Err()` before `ExitError`:
  ```go
  err := cmd.Run()
  if ctx.Err() == context.DeadlineExceeded {
      syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
      return Result{Output: "command timed out", IsError: true}, nil
  }
  ```
- **Cap timeout:** Clamp `in.Timeout` to max 600000ms.
- **Output size limit:** Cap combined stdout+stderr buffer at 1MB. Truncate with `"... (output truncated, showing last 500 lines)"`.
- **`run_in_background`:** Add `RunInBackground bool` to input struct. When true, start the process, return the PID immediately, don't wait for completion. Store running background processes in a map for later retrieval.
- **Background job notifications:** Show persistent indicator in status bar (`bg: 2 running`) and auto-notify inline when a background job finishes. More natural than Claude Code's manual task ID polling.

### 3.2 Grep schema and feature parity

**File:** `pkg/tools/grep.go`

- **Add missing parameters to schema:** `-A`, `-B`, `-C`, `-i`, `-n` (default true), `head_limit`, `offset`, `type` (file type filter).
- **Remove redundant manual parsing** of `-A`/`-B`/`-C` if struct tags handle them (verify with test).
- **Add `head_limit` and `offset`:** After collecting results, apply offset then limit before returning.
- **Add `type` filter:** Map common type names to glob patterns (e.g., `"js"` -> `"*.js"`, `"py"` -> `"*.py"`).
- **Binary file detection:** Check first 8KB for null bytes; skip binary files.
- **Context cancellation:** Check `ctx.Err()` periodically during file traversal.

### 3.3 Read improvements

**File:** `pkg/tools/read.go`

- **Default 2000-line limit:** When no `limit` is specified, cap at 2000 lines and append `"... (truncated, showing first 2000 of N lines)"`.
- **Line truncation:** Truncate lines longer than 2000 characters with `"... (line truncated)"`.

### 3.4 Write safety check

**File:** `pkg/tools/write.go`

- **Track reads:** Add a `readFiles map[string]bool` to the session or tool context. When `ReadTool.Execute` succeeds, record the path. When `WriteTool.Execute` is called, check if the path was previously read. If not, return an error: `"File has not been read in this session. Read it first to avoid accidental overwrites."` Skip check for new files (path doesn't exist).

### 3.5 Edit improvements

**File:** `pkg/tools/edit.go`

- **Preserve file permissions:** `os.Stat` before write, use original mode in `os.WriteFile`.
- **Reject no-op edits:** If `old_string == new_string`, return error `"old_string and new_string are identical"`.

### 3.6 Glob improvements

**File:** `pkg/tools/glob.go`

- **Result limit:** Default cap at 1000 results. Add optional `limit` parameter to schema.
- **Skip noise directories:** Skip `.git`, `node_modules`, `__pycache__`, `.next`, `dist`, `build` during walk unless the pattern explicitly includes them.
- **Context cancellation:** Check `ctx.Err()` in the walk callback.

### 3.7 Deterministic tool ordering

**File:** `pkg/tools/registry.go`

- **Preserve registration order:** Add `order []string` field. `Register` appends to `order`. `All()` and `Schemas()` iterate `order` instead of the map.

---

## Chunk 4: TUI Core UX [DONE] COMPLETED

**Priority:** P1 -- Bugs and missing basics that affect every session.
**Estimated scope:** ~300 lines changed across 5 files.
**Dependencies:** Chunk 1 (session struct for interrupt).
**Status:** Completed. PR #4.

### 4.1 Agent interrupt (Ctrl+C)

**Problem:** Ctrl+C kills the program. Should cancel the current turn.

**File:** `internal/tui/model.go`

**Fix:** Add interrupt state tracking:
```go
type Model struct {
    // ...
    cancelTurn context.CancelFunc // cancel function for current agent turn
}
```

In `handleKeyMsg`, when `state == StateRunning` and key is `ctrl+c`:
- Call `m.cancelTurn()` to cancel the agent's context.
- Transition to `StateInput`.
- Append a visual divider: `"(interrupted)"`.

In `handleSubmit`, create a child context:
```go
turnCtx, cancel := context.WithCancel(m.ctx)
m.cancelTurn = cancel
```

Second Ctrl+C within 1 second of the first -> `tea.Quit`.

### 4.2 Fix Escape permission deadlock

**File:** `internal/tui/model.go:249-253`

**Fix:** Change `return m, nil` to `return m, waitForAgent(m.agentCh)`.

### 4.3 Thinking/typing spinner

**Files:** `internal/tui/model.go`, `internal/tui/output.go`

- Add a `spinner` field to `Model` (use `bubbles/spinner`).
- Start spinner when entering `StateRunning`.
- Stop spinner on first `MessageTextDelta` or `MessageToolCall`.
- Render spinner in `View()` below the output: `"Claude is thinking..."`.

### 4.4 Tool result expand/collapse keybinding

**File:** `internal/tui/output.go`

- Wire `Enter` key (when not in `StateInput`) to `ToggleCollapse` on the nearest collapsed tool result block.
- Alternatively, use `e` for expand in the output viewport keymap.
- Remove the 30-line hard truncation from the expanded view (only apply to collapsed view).

### 4.5 Cost calculation per model

**File:** `internal/tui/statusbar.go`

**Fix:** Replace hardcoded Opus pricing with a lookup:
```go
var modelPricing = map[string][2]float64{
    "claude-opus-4-6":   {15.0, 75.0},
    "claude-sonnet-4-6": {3.0, 15.0},
    "claude-haiku-4-5":  {0.80, 4.0},
}
```

Use the active model to select pricing. Fall back to Opus pricing for unknown models.

### 4.6 Clean up dead code

- Remove unused `AgentHeader` and `CodeBlockBorder` styles from `styles.go`.
- Either enable thinking display (use `AppendThinking`) or remove `renderThinkingBlock` from `output.go`. Recommendation: enable it behind a `/thinking` toggle command.
- Remove unused `Height()` method from `statusbar.go`.

---

## Chunk 5: Config, Auth, and Hooks [DONE] COMPLETED

**Priority:** P1 -- Incorrect CLAUDE.md discovery affects every session.
**Estimated scope:** ~200 lines changed across 5 files.
**Dependencies:** None.
**Status:** Completed. PR #3.

### 5.1 Fix CLAUDE.md discovery

**File:** `pkg/config/claudemd.go`

- **Check bare `CLAUDE.md`:** At each directory level, check both `CLAUDE.md` and `.claude/CLAUDE.md`.
- **Collect all matches:** Return a slice of all found CLAUDE.md contents (concatenated), not just the first.
- **Support `.claude/CLAUDE.local.md`:** Check for this at each level, append if found.

Update `FindClaudeMD` signature:
```go
func FindClaudeMD(cwd string) (projectContent string, userContent string, err error)
```

Where `projectContent` is the concatenation of all project-level CLAUDE.md files found walking from cwd to root.

### 5.2 Fix hook duplication

**File:** `pkg/hooks/loader.go`

**Fix:** Track visited directories. When the walk-up loop reaches the user home directory, skip it (already loaded on line 18).

```go
userHome, _ := os.UserHomeDir()
// ... load user-level hooks ...
for dir := cwd; dir != "/"; dir = filepath.Dir(dir) {
    if dir == userHome {
        continue // already loaded user-level
    }
    // ... load hooks from dir ...
}
```

### 5.3 Fix settings zero-value merge

**File:** `pkg/config/settings.go`

**Fix:** Use `*int` for `MaxTurns` so `nil` means "not set" and `0` means "explicitly unlimited":
```go
type Settings struct {
    Model     string  `json:"model,omitempty"`
    MaxTurns  *int    `json:"max_turns,omitempty"`
    // ...
}
```

### 5.4 Fire SessionEnd hook

**File:** `internal/tui/model.go`

**Fix:** In the cleanup path (when bubbletea exits), fire the `SessionEnd` hook:
```go
// In a cleanup method or defer in main.go:
hookRunner.Run(ctx, hooks.SessionEnd, "", nil)
```

### 5.5 Enrich system prompt

**File:** `pkg/config/prompt.go`

Add environment info block to `BuildSystemPrompt`:
```
## Environment
- Platform: darwin
- Shell: zsh
- Working directory: /path/to/project
- Date: 2026-02-27
- Model: claude-opus-4-6
```

This gives the agent awareness of its runtime context, matching Claude Code's behavior.

---

## Chunk 6: MCP Improvements [DONE] COMPLETED

**Priority:** P2 -- Functional but has correctness bugs.
**Estimated scope:** ~200 lines changed across 4 files.
**Dependencies:** None.
**Status:** Completed. PR #7, PR #8 (review fixes).

### 6.1 Fix tool name prefix stripping

**File:** `pkg/mcp/adapter.go`

**Fix:** Store the original MCP tool name separately:
```go
type MCPTool struct {
    client        *Client
    qualifiedName string
    mcpName       string  // original name from the MCP server
    description   string
    schema        json.RawMessage
}
```

`MCPToolName()` returns `t.mcpName` directly.

### 6.2 Fix silent data loss on server death

**File:** `pkg/mcp/client.go`

**Fix:** Use `resp, ok := <-ch` pattern:
```go
case resp, ok := <-ch:
    if !ok {
        return fmt.Errorf("mcp server exited unexpectedly")
    }
```

### 6.3 Capture stderr

**File:** `pkg/mcp/client.go`

**Fix:** `cmd.Stderr = io.Discard` (or pipe to a log file for debugging).

### 6.4 Add environment variable support

**File:** `pkg/config/settings.go`

**Fix:** Add `Env map[string]string` to `MCPServerConfig`.

**File:** `pkg/mcp/client.go`

**Fix:** Apply env vars to `cmd.Env` during server startup:
```go
cmd.Env = append(os.Environ(), envToSlice(cfg.Env)...)
```

### 6.5 Duplicate server name guard

**File:** `pkg/mcp/manager.go`

**Fix:** Check `m.servers[name]` before starting. If exists, close the old one first or return an error.

### 6.6 MCP health visibility

Claude Code gives zero visibility into MCP server status. Glamdring should make server state visible and manageable.

**Status bar indicator:** Show connected server count (`mcp: 2`). If a server dies, show error state (`mcp: 1/2`).

**`/mcp` command:** Show server status, restart failed servers, disconnect servers.

**Inline error surfacing:** When a server dies mid-session, surface an inline notification instead of silently dropping tools.

**Files:**
- `internal/tui/statusbar.go` — MCP server count indicator
- `internal/tui/builtins.go` — `/mcp` command (list, restart, disconnect)
- `pkg/mcp/manager.go` — Health check, restart API, status reporting
- `pkg/mcp/client.go` — Death detection callback to notify TUI

### 6.7 Per-tool MCP enable/disable

Claude Code loads all tools from all MCP servers unconditionally, bloating the system prompt and wasting context window on tools the agent will never use in a given session. Glamdring should let users selectively enable/disable individual MCP tools.

**Config (`.claude/settings.json`):**
```json
{
  "mcpServers": {
    "my-server": {
      "command": "npx",
      "args": ["-y", "my-mcp-server"],
      "tools": {
        "enabled": ["tool-a", "tool-b"],
        "disabled": ["noisy-tool-c"]
      }
    }
  }
}
```

Semantics: if `enabled` is set, only those tools are registered (whitelist). If `disabled` is set, all tools except those are registered (blacklist). If neither, all tools are registered (current behavior). `enabled` takes precedence over `disabled` if both are set.

**Runtime toggle via `/mcp` command:**
```
/mcp tools my-server          — list tools with enabled/disabled status
/mcp disable my-server tool-c — disable a tool for this session
/mcp enable my-server tool-c  — re-enable a tool for this session
```

Session-level overrides don't persist to config — they reset on restart.

**Files:**
- `pkg/config/settings.go` — Add `Tools` struct to `MCPServerConfig`
- `pkg/mcp/adapter.go` — Filter tools during registration based on config
- `pkg/mcp/manager.go` — Session-level enable/disable API
- `internal/tui/builtins.go` — Extend `/mcp` command with tool subcommands

---

## Chunk 7: CLI Feature Parity [DONE] COMPLETED

**Priority:** P1 -- CLI correctness + yolo mode.
**Estimated scope:** ~150 lines across main.go, model.go, builtins.go, statusbar.go, session.go.
**Dependencies:** Chunk 1 (session struct for sessionAllow).
**Status:** Completed. PR #5.

### ~~7.1 `--print` / `-p` non-interactive mode~~ SKIPPED

### 7.2 `--version`

Add build-time version injection via `ldflags`:
```go
var version = "dev"  // overridden by -ldflags "-X main.version=v1.0.0"
```

### 7.3 SIGTERM handling

**File:** `cmd/glamdring/main.go`

**Fix:** Add `syscall.SIGTERM` to `signal.NotifyContext`.

### 7.4 Fix defer ordering

**File:** `cmd/glamdring/main.go`

**Fix:** Ensure `cancel()` is the last defer (runs first):
```go
ctx, cancel := signal.NotifyContext(...)
defer cancel() // runs first (LIFO)

mcpMgr := mcp.NewManager()
defer mcpMgr.Close() // runs second

// ... indexDB ...
defer indexDB.Close() // runs third
```

Wait -- this is already the correct order for LIFO. The issue is that `cancel()` is deferred first (line 85), so it runs last. Fix: move `defer cancel()` to after all other defers, or explicitly call `cancel()` at the start of the MCP/index cleanup:

```go
defer func() {
    cancel()        // signal everything to stop
    mcpMgr.Close()  // then clean up resources
    if indexDB != nil {
        indexDB.Close()
    }
}()
```

### 7.5 Yolo mode

Binary permission bypass — Normal or Yolo. No 4-mode cycle, no scary flag names.

**Three entry points:**

1. **`--yolo` flag** at launch — auto-approve all tools from the start.
   ```go
   // cmd/glamdring/main.go
   yolo := flag.Bool("yolo", false, "auto-approve all tool permissions")
   ```

2. **`Shift+Tab` keybinding** — toggles yolo on/off mid-session. Same key both directions.
   ```go
   // internal/tui/model.go — in handleKeyMsg
   case key.Matches(msg, key.NewBinding(key.WithKeys("shift+tab"))):
       m.session.ToggleYolo()
       return m, nil
   ```

3. **`/yolo` slash command** — alternative to keybinding. Supports optional scoping.
   ```go
   // internal/tui/builtins.go
   // "/yolo"           → toggle all tools
   // "/yolo bash,write" → toggle only those tools
   ```

**Session integration:**
```go
// pkg/agent/session.go
func (s *Session) ToggleYolo() {
    s.yolo = !s.yolo
    if s.yolo {
        for _, t := range s.registry.All() {
            s.sessionAllow[t.Name()] = true
        }
    } else {
        s.sessionAllow = make(map[string]bool)
    }
}

func (s *Session) SetYolo(on bool)  // for --yolo flag at startup
func (s *Session) IsYolo() bool
```

**Scoped yolo (`/yolo bash,write`):**
```go
func (s *Session) SetYoloScoped(tools []string) {
    for _, name := range tools {
        s.sessionAllow[name] = true
    }
}
```

**Status bar indicator:**
```go
// internal/tui/statusbar.go
// When yolo is active, append "YOLO" in warning color to status bar
if s.session != nil && s.session.IsYolo() {
    parts = append(parts, warningStyle.Render("YOLO"))
}
```

**Files:**
- `cmd/glamdring/main.go` — `--yolo` flag, pass to session via config
- `pkg/agent/session.go` — `yolo bool`, `ToggleYolo()`, `SetYolo()`, `IsYolo()`, `SetYoloScoped()`
- `pkg/agent/config.go` — `Yolo bool` field in Config
- `internal/tui/model.go` — Shift+Tab keybinding
- `internal/tui/builtins.go` — `/yolo` command with optional scope
- `internal/tui/statusbar.go` — YOLO indicator

---

## Chunk 8: Performance [DONE] COMPLETED

**Priority:** P2 -- Noticeable in long sessions.
**Estimated scope:** ~200 lines changed across 3 files.
**Dependencies:** None.
**Status:** Completed. PR TBD. Item 8.4 was completed in Chunk 3 (PR #6).

### 8.1 Cache rendered output blocks

**File:** `internal/tui/output.go`

**Fix:** Add a `rendered string` field to `outputBlock`. When a block is finalized (next block starts, or `MessageDone`), cache its rendered output and skip glamour on subsequent `rerender()` calls. Only re-render the active (last) text block.

```go
type outputBlock struct {
    kind      blockKind
    content   string
    collapsed bool
    finalized bool   // true when block is complete
    rendered  string // cached rendered output
}
```

In `rerender()`:
```go
for i, b := range m.blocks {
    if b.finalized && b.rendered != "" {
        parts = append(parts, b.rendered)
    } else {
        rendered := renderBlock(b)
        if b.finalized {
            m.blocks[i].rendered = rendered
        }
        parts = append(parts, rendered)
    }
}
```

### 8.2 Debounce glamour renderer recreation

**File:** `internal/tui/output.go`

**Fix:** Don't recreate the renderer in `SetSize`. Instead, set a dirty flag and recreate lazily in the next `rerender()` call.

### 8.3 Tool result truncation for API

**File:** `pkg/agent/agent.go`

**Fix:** Before appending tool results to the conversation, truncate outputs exceeding a threshold (e.g., 50KB) with a summary message. This prevents blowing the context window with huge grep results or file reads.

```go
const maxToolResultSize = 50_000
if len(toolResult.Output) > maxToolResultSize {
    toolResult.Output = toolResult.Output[:maxToolResultSize] +
        "\n... (truncated, full output was " + strconv.Itoa(len(toolResult.Output)) + " bytes)"
}
```

**Transparent truncation in TUI:** Claude Code silently truncates internally. Glamdring should show `"showing 50KB of 200KB — press 'e' for full output"` in the collapsed tool result view. The user knows what happened and can act on it.

### 8.4 Glob result limit

Already covered in Chunk 3.6 but has performance implications. Default 1000 results with skip for `.git`, `node_modules`, etc.

---

## Chunk 9: Differentiation Features

**Priority:** P1-P3 mixed -- Differentiation features that set glamdring apart.
**Estimated scope:** Variable per feature.
**Dependencies:** Chunks 1-4.

### 9.1 Conversation export

**Command:** `/export [path]`

Dump the full conversation history as markdown. Each turn gets a header (`## User` / `## Assistant`), tool calls rendered as fenced code blocks, thinking blocks as collapsed `<details>` sections.

Also support `/export --html [path]` for a self-contained, shareable HTML file with syntax highlighting, collapsible thinking blocks, and styled output. Claude Code has no export capability at all.

**Files:**
- `internal/tui/builtins.go` — `/export` command
- `internal/tui/export.go` (new) — Markdown and HTML renderers for conversation history

### 9.2 Thinking toggle

**Command:** `/thinking [on|off]`

Toggle display of thinking blocks in the output. When on, `MessageThinkingDelta` is routed to `output.AppendThinking` (already implemented but not wired up). Default: off.

### 9.3 Input history with reverse search

Up/down arrow to cycle through previous prompts. Store last N prompts in a ring buffer.

**`Ctrl+R` reverse search:** Match terminal muscle memory. Opens an inline search prompt that filters through previous inputs as you type, like bash/zsh. More natural for terminal users than bare up/down.

**Files:**
- `internal/tui/input.go` — Up/down history cycling, Ctrl+R search mode
- `internal/tui/history.go` (new) — Ring buffer with substring search

### ~~9.4 Per-turn cost display~~ SKIPPED

### 9.5 Configurable permission presets (path-scoped)

Claude Code has flat `allowedTools` in settings. Glamdring supports path-scoped and command-scoped permission rules — more powerful, less yolo-or-nothing.

**File:** `.claude/permissions.json` (new config format):
```json
{
  "allow": [
    {"tool": "Write", "path": "src/**"},
    {"tool": "Bash", "command": "go test*"},
    {"tool": "Bash", "command": "go build*"}
  ],
  "deny": [
    {"tool": "Bash", "command": "rm -rf*"}
  ]
}
```

**File:** `pkg/agent/agent.go` — Extend `isAllowed` to check glob patterns against tool inputs. Path rules match against the file path argument. Command rules match against the bash command string.

### 9.6 Context window usage display

**Priority: P1** — Prevents the single worst Claude Code UX failure (hitting the context wall with no warning).

Show live context usage in the status bar. Estimate from cumulative token counts vs model's context limit:

```
claude-opus-4-6 │ ctx: 34% │ $0.47
```

At 60%, suggest `/compact` with an inline hint. At 80%, show a warning color. This gives the user agency over context management instead of a surprise wall.

**Model context limits:**
```go
var modelContextLimits = map[string]int{
    "claude-opus-4-6":   200000,
    "claude-sonnet-4-6": 200000,
    "claude-haiku-4-5":  200000,
}
```

**Context threshold hooks:** Extend the hook system with a `ContextThreshold` event that fires when context usage crosses a configured percentage. Enables automated responses — auto-compact, notifications, logging.

```json
{
  "hooks": [
    {
      "event": "ContextThreshold",
      "threshold": 0.6,
      "actions": ["/lembas", "/compact"]
    },
    {
      "event": "ContextThreshold",
      "threshold": 0.8,
      "command": "notify-send 'glamdring: context critical'"
    }
  ]
}
```

Each threshold fires once per crossing (not repeatedly). Reset when context drops (e.g., after `/compact`). The built-in 60% hint and 80% warning color are defaults that work without any hook config — hooks are for users who want custom automation on top.

**Hook actions (internal commands):** All hook events support `actions` (internal slash commands/skills) alongside `command` (shell). This applies to every hook event, not just ContextThreshold:

```json
{
  "hooks": [
    {
      "event": "ContextThreshold",
      "threshold": 0.6,
      "actions": ["/lembas", "/compact"]
    },
    {
      "event": "SessionStart",
      "actions": ["/council"]
    },
    {
      "event": "SessionEnd",
      "command": "echo 'session ended' >> ~/glamdring.log"
    }
  ]
}
```

- `command`: shell command (existing behavior, unchanged)
- `actions`: array of internal slash commands/skills, executed sequentially
- Both can coexist on the same hook — `actions` run first, then `command`
- Actions receive the same context as manual invocation (current session, conversation state)

This is a major differentiator — Claude Code hooks can only run shell commands. Glamdring hooks can trigger internal workflows, enabling fully automated context management (`/lembas` + `/compact` at threshold), auto-onboarding (`/council` at session start), and custom skill chains.

**Files:**
- `internal/tui/statusbar.go` — Context percentage calculation and display
- `internal/tui/model.go` — Compact suggestion hint at threshold, threshold crossing detection
- `pkg/hooks/hooks.go` — Add `ContextThreshold` event type, add `Actions []string` field to hook config
- `pkg/hooks/runner.go` — Fire hook with context percentage as env var (`GLAMDRING_CONTEXT_PCT`), execute internal actions via command dispatcher

### 9.7 Bash output streaming

Stream Bash tool output to the TUI as it arrives instead of buffering until completion. This requires changing `BashTool` from a synchronous `Execute` to a streaming pattern, or using a callback.

**File:** `pkg/tools/bash.go` -- Use `cmd.StdoutPipe()` and `cmd.StderrPipe()`, read line-by-line, emit partial results via a callback or channel.

**File:** `internal/tui/output.go` -- Render partial tool results as they arrive.

This is a larger change that touches the `Tool` interface (needs a streaming variant). Consider as a follow-up to the core chunks.

---

## Chunk 10: Agent Teams

**Priority:** P2 -- Enables multi-agent coordination (fellowship-style workflows).
**Estimated scope:** ~600-800 lines across 6-8 new files.
**Dependencies:** Chunk 1 (session struct), Chunk 7 (subagent infrastructure).

### 10.1 Team lifecycle tools

New tools: `TeamCreate`, `TeamDelete`. Teams are named collections of agents with a shared task list and message bus.

**Files:**
- `pkg/tools/team_create.go` (new) -- Creates a team config at `~/.claude/teams/{name}/config.json` with member registry.
- `pkg/tools/team_delete.go` (new) -- Cleans up team config and task directory.
- `pkg/teams/team.go` (new) -- Team struct: name, members, config path. Handles member registration/removal.

### 10.2 Task management tools

New tools: `TaskCreate`, `TaskList`, `TaskGet`, `TaskUpdate`. Persistent task list per team stored as JSON files.

**Files:**
- `pkg/tools/task_create.go` (new) -- Creates tasks with subject, description, status, owner, dependencies (blocks/blockedBy).
- `pkg/tools/task_list.go` (new) -- Lists all tasks with summary view.
- `pkg/tools/task_get.go` (new) -- Full task detail by ID.
- `pkg/tools/task_update.go` (new) -- Update status, owner, metadata, dependencies.
- `pkg/teams/tasks.go` (new) -- Task storage: read/write JSON files in `~/.claude/tasks/{team-name}/`.

### 10.3 Inter-agent messaging

New tool: `SendMessage`. Supports direct messages, broadcasts, shutdown requests/responses, and plan approval flows.

**Files:**
- `pkg/tools/send_message.go` (new) -- Message routing: DM to named agent, broadcast to all, shutdown protocol.
- `pkg/teams/mailbox.go` (new) -- Per-agent message queue. Agents poll or get notified on new messages.

### 10.4 Team-aware Task tool

Extend the existing Task tool to support `team_name` and `name` parameters so spawned subagents join a team and can be messaged by name.

**Files:**
- `pkg/tools/task.go` -- Add `team_name` and `name` fields to input schema. Pass team context to spawned agent sessions.
- `pkg/agent/session.go` -- Add optional team membership to Session. Wire mailbox delivery into the agent loop.

### 10.5 Agent idle/resume lifecycle

Agents go idle between turns and can be woken by incoming messages. Requires a message delivery mechanism that interrupts idle agents.

**Files:**
- `pkg/teams/lifecycle.go` (new) -- Idle state tracking, wake-on-message, shutdown protocol handling.
- `pkg/agent/session.go` -- Hook message delivery into Turn() so incoming messages can trigger new turns.

---

## Implementation Order

```
Chunk 1 (Agent Loop)   [DONE] ─┐
Chunk 2 (API Client)   [DONE] ─┤
Chunk 3 (Tools)        [DONE] ─┼──→  Chunk 4 (TUI UX)   [DONE] ──→  Chunk 9 (Differentiation)
Chunk 5 (Config/Auth)  [DONE] ─┤
Chunk 6 (MCP)          [DONE] ─┤                                  Chunk 8 (Performance)  [DONE]
Chunk 7 (CLI Parity)   [DONE] ─┘
                         │
                         └──→  Chunk 10 (Agent Teams) -- depends on 1 + 7
```

Chunks 1-8 are complete. Remaining chunks 9 and 10 can be done in any order -- all dependencies are satisfied.

## Test Plan

Each chunk should include tests:

| Chunk | Required Tests |
|-------|---------------|
| 1 | Session multi-turn round-trip, stop reason handling, thinking signature preservation |
| 2 | SSE parser edge cases (large payloads, malformed events), retry logic, goroutine leak regression |
| 3 | Bash timeout detection, Grep schema parameter parsing, Read line limits, Edit permission preservation, Write safety check |
| 4 | State transitions (interrupt flow, permission deny/escape), spinner lifecycle |
| 5 | CLAUDE.md discovery (bare + nested + local), hook deduplication, settings merge with zero values |
| 6 | MCP tool name with underscores, server death handling, env var propagation, health status reporting |
| 7 | Yolo toggle (on/off/scoped), --yolo flag, status bar indicator, version flag |
| 8 | Render cache hit rate, truncation correctness, transparent truncation display |
| 10 | Team create/delete lifecycle, task CRUD, message delivery (DM + broadcast), shutdown protocol, agent idle/resume |

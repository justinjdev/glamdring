## Context

Glamdring currently hardcodes `.claude/` paths across six packages: `pkg/config` (settings, claudemd, permissions), `pkg/hooks`, `pkg/commands`, `pkg/agents`, and `pkg/auth`. Each package independently constructs `.claude/`-relative paths. There is no shared path resolution -- each loader has its own walk-up-the-tree or home-dir logic.

The auth package is a special case: it reads `~/.claude.json` (shared OAuth tokens with Claude Code) and `~/.claude/.auth-lock`. These paths are part of Claude Code's OAuth protocol and must remain compatible.

## Goals / Non-Goals

**Goals:**
- Establish `.glamdring/` as the primary config directory at both project and user levels
- Fall back to `.claude/` equivalents when `.glamdring/` files are not found
- Centralize path resolution so all loaders use a shared function
- Use `~/.config/glamdring/` for user-level config (XDG-compliant)
- Support `GLAMDRING.md` as the primary instructions filename alongside `CLAUDE.md` fallback
- Keep auth paths compatible with Claude Code's OAuth flow

**Non-Goals:**
- XDG_CONFIG_HOME environment variable support (can add later)
- Migration tool or CLI command to move `.claude/` to `.glamdring/`
- Removing `.claude/` support entirely
- Changing the config file format or schema
- Renaming `settings.json` fields or struct names in Go code

## Decisions

### 1. Centralized path resolution via `configpaths` package

Create a new `pkg/config/paths.go` (or a small `pkg/configpaths` package) that provides path resolution for all config lookups. Every loader currently has its own path logic -- this unifies it.

```go
// Resolve checks .glamdring/<rel> first, then .claude/<rel> as fallback.
// Returns the first path that exists, or empty string.
func Resolve(baseDir, rel string) string

// ResolveDir checks .glamdring/<rel>/ first, then .claude/<rel>/ as fallback.
func ResolveDir(baseDir, rel string) string

// UserConfigDir returns ~/.config/glamdring, falling back to ~/.claude.
func UserConfigDir() string

// ProjectRoot walks up from cwd looking for .glamdring/ or .claude/.
func ProjectRoot(cwd string) string
```

All existing loaders call these functions instead of hardcoding `.claude/`.

**Alternative considered:** Adding fallback logic inline in each loader. Rejected because it duplicates the priority chain 6+ times and makes it fragile to change.

### 2. File and directory naming conventions

| Purpose | Primary | Fallback |
|---|---|---|
| Project config dir | `.glamdring/` | `.claude/` |
| Project settings | `.glamdring/config.json` | `.claude/settings.json` |
| Project permissions | `.glamdring/permissions.json` | `.claude/permissions.json` |
| Project instructions | `GLAMDRING.md`, `.glamdring/GLAMDRING.md`, `.glamdring/GLAMDRING.local.md` | `CLAUDE.md`, `.claude/CLAUDE.md`, `.claude/CLAUDE.local.md` |
| Project commands | `.glamdring/commands/` | `.claude/commands/` |
| Project agents | `.glamdring/agents/` | `.claude/agents/` |
| Project hooks | `.glamdring/config.json` `hooks` key | `.claude/settings.json` `hooks` key |
| User config dir | `~/.config/glamdring/` | `~/.claude/` |
| User settings | `~/.config/glamdring/config.json` | `~/.claude/settings.json` |
| User instructions | `~/.config/glamdring/GLAMDRING.md` | `~/.claude/CLAUDE.md` |
| User commands | `~/.config/glamdring/commands/` | `~/.claude/commands/` |
| User agents | `~/.config/glamdring/agents/` | `~/.claude/agents/` |

Settings file is renamed to `config.json` (not `settings.json`) since we're establishing a new convention. The Go struct stays `Settings` internally.

**Alternative considered:** Keeping the filename as `settings.json` under `.glamdring/`. Rejected -- if we're making a clean namespace, use a cleaner name. `config.json` is more standard (matches opencode's `opencode.json` pattern).

### 3. Resolution priority

For each config file, the resolution order is:

1. `.glamdring/` equivalent (primary)
2. `.claude/` equivalent (fallback)

When both exist, `.glamdring/` wins entirely (no merging across namespaces). This avoids confusing behavior where settings from two directories interact.

For settings specifically, the full chain is:
1. Defaults
2. User-level (`~/.config/glamdring/config.json` or `~/.claude/settings.json`)
3. Project-level (`.glamdring/config.json` or `.claude/settings.json`)

Each level resolves its own primary/fallback independently before merging.

**Alternative considered:** Merging `.glamdring/` and `.claude/` settings at each level. Rejected -- it creates confusing precedence and makes it hard to reason about which file a setting came from.

### 4. Instructions file: check both namespaces, concatenate

Unlike settings (where one file wins), instructions files are additive -- multiple files at different directory levels are concatenated. The discovery order at each directory level becomes:

1. `GLAMDRING.md` (bare)
2. `.glamdring/GLAMDRING.md`
3. `.glamdring/GLAMDRING.local.md`
4. `CLAUDE.md` (bare, fallback)
5. `.claude/CLAUDE.md` (fallback)
6. `.claude/CLAUDE.local.md` (fallback)

All found files are concatenated. This is intentionally additive because a project might have a `CLAUDE.md` checked into git (for Claude Code users) and a `GLAMDRING.md` with glamdring-specific instructions. Both are useful.

**Alternative considered:** Only check one namespace per directory level (if `GLAMDRING.md` exists, skip `CLAUDE.md`). Rejected -- it breaks projects that want to support both tools with shared instructions in `CLAUDE.md` and glamdring-specific additions in `GLAMDRING.md`.

### 5. Auth paths: keep as-is

Auth tokens (`~/.claude.json`) and the auth lock (`~/.claude/.auth-lock`) stay at their current paths. These are part of Claude Code's OAuth flow -- glamdring uses the same OAuth provider (platform.claude.com) and sharing tokens means users don't need to log in twice. Moving these would break the shared auth story for no benefit.

### 6. FindProjectRoot: check both directories

`FindProjectRoot` currently walks up looking for `.claude/`. Updated to check `.glamdring/` first, then `.claude/`. If both exist at different levels, the one found first (closest to cwd) wins.

## Risks / Trade-offs

**[Two config directories in same project]** Users who partially migrate may have config split across `.glamdring/` and `.claude/`. **Mitigation:** Log which file was loaded at startup so users can see where config is coming from. Document the migration path clearly.

**[Instructions concatenation bloat]** Checking 6 file paths per directory level (3 glamdring + 3 claude) could accumulate duplicate content if projects have both. **Mitigation:** This is a feature, not a bug -- users control what's in each file. If they duplicate content, that's their choice.

**[Ecosystem confusion]** `.glamdring/` is a new convention that Claude Code users won't recognize. **Mitigation:** Fallback ensures pure `.claude/` projects work without changes. Only users who want glamdring-specific config need to create `.glamdring/`.

**[Auth not migrated]** Keeping auth at `~/.claude.json` means glamdring still depends on a `.claude` path for one thing. **Mitigation:** This is a shared resource, not glamdring's config. It's analogous to multiple Git clients sharing `~/.gitconfig`. Can revisit if glamdring adds its own auth provider.

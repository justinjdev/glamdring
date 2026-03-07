## Why

Glamdring currently reads all configuration from `.claude/` (settings.json, permissions.json, CLAUDE.md, commands/, agents/) and `~/.claude/` for user-level config. This creates an implicit dependency on Claude Code's directory conventions and causes confusion when both tools are used in the same project. Glamdring is diverging with its own features (teams, workflows, experimental flags) that have no Claude Code equivalent -- these don't belong in Claude Code's namespace. Glamdring should own its config namespace while maintaining backward compatibility with `.claude/` for migration.

## What Changes

- **BREAKING**: Primary config directory changes from `.claude/` to `.glamdring/` at both project and user levels
- **BREAKING**: Primary settings file changes from `.claude/settings.json` to `.glamdring/config.json`
- **BREAKING**: Primary instructions file changes from `CLAUDE.md` / `.claude/CLAUDE.md` to `GLAMDRING.md` / `.glamdring/GLAMDRING.md`
- Add fallback: when a `.glamdring/` file is not found, check the corresponding `.claude/` location
- `.glamdring/` takes priority over `.claude/` when both exist
- Permissions, commands, agents, and hooks all move to `.glamdring/` with `.claude/` fallback
- User-level config moves from `~/.claude/` to `~/.config/glamdring/` (XDG-compliant) with `~/.claude/` fallback

## Capabilities

### New Capabilities
- `config-namespace`: Resolution logic for the `.glamdring/` -> `.claude/` fallback chain, XDG-compliant user config paths, and the file/directory naming conventions

### Modified Capabilities
- `config-loader`: Settings, CLAUDE.md, and working directory discovery all change to check `.glamdring/` first with `.claude/` fallback
- `permission-system`: Permissions file path changes from `.claude/permissions.json` to `.glamdring/permissions.json` with fallback
- `hooks`: Hook loader changes from `.claude/hooks.json` to `.glamdring/hooks.json` with fallback
- `commands`: Command loader changes from `.claude/commands/` to `.glamdring/commands/` with fallback
- `custom-agents`: Agent loader changes from `.claude/agents/` to `.glamdring/agents/` with fallback

## Impact

- `pkg/config/settings.go` -- settings file discovery paths
- `pkg/config/claudemd.go` -- instruction file discovery (GLAMDRING.md + CLAUDE.md fallback)
- `pkg/config/permissions.go` -- permissions file path
- `pkg/hooks/loader.go` -- hooks file path
- `pkg/commands/loader.go` -- commands directory path
- `pkg/agents/loader.go` -- agents directory path
- `pkg/auth/` -- credential storage paths (login, logout, store, resolve)
- All corresponding test files
- README.md -- documentation of config locations
- CLAUDE.md for this project (meta: this project's own instructions file)

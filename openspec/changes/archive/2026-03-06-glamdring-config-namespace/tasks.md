## 1. Centralized path resolution

- [x] 1.1 Create `pkg/config/paths.go` with `Resolve(baseDir, rel string) string` that checks `.glamdring/<rel>` then `.claude/<rel>`, returning first existing path
- [x] 1.2 Add `ResolveDir(baseDir, rel string) string` for directory resolution (`.glamdring/<rel>/` then `.claude/<rel>/`)
- [x] 1.3 Add `UserConfigDir() string` returning `~/.config/glamdring/` if it exists, else `~/.claude/`
- [x] 1.4 Add `ResolveUserFile(rel string) string` that checks `~/.config/glamdring/<rel>` then `~/.claude/<rel>`
- [x] 1.5 Update `FindProjectRoot` in `pkg/config/claudemd.go` to check `.glamdring/` before `.claude/`
- [x] 1.6 Add tests in `pkg/config/paths_test.go` for all resolution functions with primary, fallback, both-exist, and neither-exist scenarios

## 2. Settings loader migration

- [x] 2.1 Update `loadUserSettings()` in `pkg/config/settings.go` to use `ResolveUserFile("config.json")` with fallback to `ResolveUserFile("settings.json")`
- [x] 2.2 Update `loadProjectSettings()` in `pkg/config/settings.go` to use `Resolve(dir, "config.json")` with fallback to `Resolve(dir, "settings.json")` at each walk-up level
- [x] 2.3 Update tests in `pkg/config/settings_test.go` for new paths

## 3. Instructions file migration

- [x] 3.1 Update `findProjectClaudeMD()` in `pkg/config/claudemd.go` to check `GLAMDRING.md`, `.glamdring/GLAMDRING.md`, `.glamdring/GLAMDRING.local.md` before the `CLAUDE.md` equivalents at each directory level
- [x] 3.2 Update `findUserClaudeMD()` to check `~/.config/glamdring/GLAMDRING.md` before `~/.claude/CLAUDE.md`
- [x] 3.3 Rename `FindClaudeMD` to `FindInstructions` (update all call sites)
- [x] 3.4 Update tests in `pkg/config/claudemd_test.go` for new file paths and mixed-namespace concatenation

## 4. Permissions migration

- [x] 4.1 Update `LoadPermissions()` in `pkg/config/permissions.go` to use `Resolve(cwd, "permissions.json")`
- [x] 4.2 Update tests in `pkg/config/permissions_test.go`

## 5. Hooks loader migration

- [x] 5.1 Update `LoadHooks()` in `pkg/hooks/loader.go` to check `.glamdring/config.json` before `.claude/settings.json` at both user and project levels
- [x] 5.2 Update tests in `pkg/hooks/loader_test.go`

## 6. Commands loader migration

- [x] 6.1 Update `Discover()` in `pkg/commands/loader.go` to use `ResolveDir` for `commands/` at both project and user levels
- [x] 6.2 Update tests in `pkg/commands/loader_test.go`

## 7. Agents loader migration

- [x] 7.1 Update `Discover()` in `pkg/agents/loader.go` to use `ResolveDir` for `agents/` at both project and user levels
- [x] 7.2 Update tests in `pkg/agents/loader_test.go`

## 8. Verification

- [x] 8.1 `go build ./...` clean
- [x] 8.2 `go test -race -count=1 ./...` passes
- [x] 8.3 `go vet ./...` clean
- [x] 8.4 Verify fallback: project with only `.claude/` works unchanged
- [x] 8.5 Verify primary: project with `.glamdring/config.json` loads correctly
- [x] 8.6 Update README.md config section to document `.glamdring/` paths and fallback behavior

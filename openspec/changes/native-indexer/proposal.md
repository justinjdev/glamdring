## Why

Glamdring currently has no awareness of repository structure beyond the working directory. The agent can only explore code through Glob, Grep, and Read — brute-force tools that don't understand packages, dependencies, or symbols. [Shire](https://github.com/justinjdev/shire) solves this as an external MCP server, but requiring MCP configuration and server management adds friction. Adding Go bindings that read shire's SQLite index directly gives the agent structured codebase knowledge as built-in tools — zero MCP config, automatic detection, native query speed.

## What Changes

- Add a read-only Go query layer that opens shire's `.shire/index.db` and executes the same FTS5/SQL queries shire's MCP tools use.
- Register 13 built-in tools (search_packages, get_package, list_packages, package_dependencies, package_dependents, dependency_graph, search_symbols, get_package_symbols, get_symbol, get_file_symbols, search_files, list_package_files, index_status) when an index is detected at startup.
- Shell out to `shire build` for index creation and rebuilds — no need to reimplement discovery, parsing, or symbol extraction.
- Add a `/index` built-in command for status display and triggering rebuilds.

## Capabilities

### New Capabilities
- `code-search`: Read-only Go query layer for shire's SQLite index, 13 built-in agent tools for searching packages/symbols/files, querying dependency graphs, and inspecting package APIs

### Modified Capabilities
- `tool-system`: New built-in tools conditionally registered when `.shire/index.db` is detected
- `tui`: New `/index` built-in command for status display and rebuild via `shire build`

## Impact

- New dependency: `modernc.org/sqlite` (pure-Go SQLite, no CGO)
- New package: `pkg/index/` (database open, query functions, tool implementations)
- Modified: `pkg/tools/` (conditional tool registration), `internal/tui/` (`/index` command)
- Reads existing `.shire/index.db` from the working directory (created by `shire build`)
- Requires `shire` binary on PATH for rebuilds (graceful degradation if not available)

## 1. Database Layer

- [x] 1.1 Add `modernc.org/sqlite` dependency and create `pkg/index/` package
- [x] 1.2 Implement `Open(dbPath string) (*DB, error)` — open existing `.shire/index.db` in read-only WAL mode, verify schema version from `shire_meta` table
- [x] 1.3 Implement query functions: `SearchPackages`, `GetPackage`, `ListPackages`
- [x] 1.4 Implement query functions: `PackageDependencies`, `PackageDependents`, `DependencyGraph` (BFS traversal)
- [x] 1.5 Implement query functions: `SearchSymbols`, `GetPackageSymbols`, `GetSymbol`, `GetFileSymbols`
- [x] 1.6 Implement query functions: `SearchFiles`, `ListPackageFiles`
- [x] 1.7 Implement query function: `IndexStatus` (read from `shire_meta` table + count queries)

## 2. Tool Implementations

- [x] 2.1 Create tool wrapper structs implementing the `Tool` interface for each of the 13 query functions
- [x] 2.2 Define JSON schemas for each tool's input parameters (matching shire's MCP tool schemas)
- [x] 2.3 Implement JSON result formatting for each tool's output

## 3. Startup Integration

- [x] 3.1 Add index detection in startup: check for `.shire/index.db` in CWD, open database if present
- [x] 3.2 Conditionally register index tools in the tool registry when database is available
- [x] 3.3 Pass database handle through `agent.Config` (new optional field)

## 4. TUI Integration

- [x] 4.1 Implement `/index` built-in command — display status from `IndexStatus` query
- [x] 4.2 Implement `/index rebuild` — shell out to `shire build --root <cwd>`, reopen database, re-register tools
- [x] 4.3 Handle shire-not-on-PATH gracefully with install instructions
- [x] 4.4 Post-turn auto-rebuild — after agent turns with file-modifying tools (Edit/Write/Bash), async rebuild index and reopen DB
- [x] 4.5 Configurable indexer — `IndexerConfig` in settings (enabled, command, auto_rebuild) with auto-detect default

## 5. Tests

- [ ] 5.1 Create a fixture `.shire/index.db` with known packages, dependencies, symbols, and files
- [ ] 5.2 Unit tests for all query functions against the fixture database
- [ ] 5.3 Unit tests for tool wrappers (JSON schema correctness, input parsing, output formatting)
- [ ] 5.4 Test conditional tool registration (present vs absent database)
- [ ] 5.5 Test `/index` command output formatting

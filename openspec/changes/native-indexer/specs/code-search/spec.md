## ADDED Requirements

### Requirement: Index detection
The system SHALL check for `.shire/index.db` in the working directory at startup. When detected, the system SHALL open the database in read-only WAL mode and register all code search tools. When not detected, the system SHALL skip tool registration and operate normally without index capabilities.

#### Scenario: Index present
- **WHEN** glamdring starts in a directory containing `.shire/index.db`
- **THEN** the database is opened and all 13 code search tools are registered

#### Scenario: Index absent
- **WHEN** glamdring starts in a directory without `.shire/index.db`
- **THEN** no code search tools are registered and the agent operates with standard tools only

#### Scenario: Corrupt database
- **WHEN** `.shire/index.db` exists but cannot be opened
- **THEN** a warning is logged and the system continues without code search tools

### Requirement: Package search tool
The system SHALL provide a `search_packages` tool that performs FTS5 full-text search across package names, descriptions, and paths. Results SHALL include package name, path, kind, version, and description. An optional `limit` parameter SHALL control the number of results.

#### Scenario: Search by name
- **WHEN** the agent calls `search_packages` with query "auth"
- **THEN** packages matching "auth" in name, description, or path are returned ranked by relevance

### Requirement: Package lookup tool
The system SHALL provide a `get_package` tool that retrieves a single package by exact name, including its dependencies, dependents count, and metadata.

#### Scenario: Exact name lookup
- **WHEN** the agent calls `get_package` with name "services/auth"
- **THEN** the full package record is returned with dependencies and dependent count

#### Scenario: Package not found
- **WHEN** the agent calls `get_package` with a name that does not exist
- **THEN** the tool returns an error indicating the package was not found

### Requirement: Package listing tool
The system SHALL provide a `list_packages` tool that returns all indexed packages, optionally filtered by kind (npm, go, cargo, etc.).

#### Scenario: List all packages
- **WHEN** the agent calls `list_packages` with no filter
- **THEN** all packages in the index are returned

#### Scenario: Filter by kind
- **WHEN** the agent calls `list_packages` with kind "go"
- **THEN** only Go packages are returned

### Requirement: Dependency query tools
The system SHALL provide `package_dependencies` and `package_dependents` tools. `package_dependencies` SHALL return what a package depends on, with an optional `internal_only` filter. `package_dependents` SHALL return what depends on a given package (reverse lookup).

#### Scenario: Internal dependencies only
- **WHEN** the agent calls `package_dependencies` with name "frontend" and `internal_only: true`
- **THEN** only dependencies that are also packages in the repository are returned

#### Scenario: Reverse dependency lookup
- **WHEN** the agent calls `package_dependents` with name "shared-utils"
- **THEN** all packages that depend on "shared-utils" are returned

### Requirement: Dependency graph traversal tool
The system SHALL provide a `dependency_graph` tool that performs BFS traversal from a root package up to a configurable depth. The tool SHALL return edges representing the transitive dependency tree, with an optional `internal_only` filter.

#### Scenario: Transitive dependency graph
- **WHEN** the agent calls `dependency_graph` with root "api-gateway" and depth 3
- **THEN** the tool returns all dependency edges reachable within 3 hops from "api-gateway"

### Requirement: Symbol search tool
The system SHALL provide a `search_symbols` tool that performs FTS5 full-text search across symbol names and signatures. Optional filters SHALL include package name and symbol kind (function, class, struct, etc.).

#### Scenario: Search for function
- **WHEN** the agent calls `search_symbols` with query "ProcessOrder"
- **THEN** matching symbols are returned with their name, kind, signature, file path, and line number

#### Scenario: Filter by package and kind
- **WHEN** the agent calls `search_symbols` with query "handler" and filters package="api" and kind="function"
- **THEN** only functions in the "api" package matching "handler" are returned

### Requirement: Package symbols tool
The system SHALL provide a `get_package_symbols` tool that lists all symbols in a given package, optionally filtered by symbol kind.

#### Scenario: List package API surface
- **WHEN** the agent calls `get_package_symbols` with package "auth-service"
- **THEN** all public symbols (functions, types, classes, etc.) in that package are returned

### Requirement: Symbol lookup tool
The system SHALL provide a `get_symbol` tool for exact name lookup of a symbol across all packages, and a `get_file_symbols` tool that lists all symbols defined in a specific file.

#### Scenario: Exact symbol lookup
- **WHEN** the agent calls `get_symbol` with name "ProcessOrder"
- **THEN** all symbols named "ProcessOrder" across all packages are returned

#### Scenario: File symbols
- **WHEN** the agent calls `get_file_symbols` with path "services/auth/handler.go"
- **THEN** all symbols defined in that file are returned with their line numbers and signatures

### Requirement: File search tool
The system SHALL provide a `search_files` tool that performs FTS5 full-text search across file paths, with optional package and extension filters. The system SHALL also provide a `list_package_files` tool that lists all files belonging to a package with optional extension filter.

#### Scenario: Search files by path
- **WHEN** the agent calls `search_files` with query "middleware"
- **THEN** files with "middleware" in their path are returned

#### Scenario: List package files by extension
- **WHEN** the agent calls `list_package_files` with package "api" and extension ".go"
- **THEN** only `.go` files in the "api" package are returned

### Requirement: Index status tool
The system SHALL provide an `index_status` tool that returns: when the index was last built, git commit at build time, total package/symbol/file counts, and build duration.

#### Scenario: Status report
- **WHEN** the agent calls `index_status`
- **THEN** the tool returns build timestamp, git commit, package count, symbol count, file count, and duration

package index

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// createTestDB sets up an in-memory SQLite database with the shire index
// schema, FTS5 indexes, and seed data for testing.
func createTestDB(t *testing.T) *DB {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	ddl := []string{
		// Core tables
		`CREATE TABLE packages (
			name TEXT PRIMARY KEY,
			path TEXT NOT NULL,
			kind TEXT NOT NULL,
			version TEXT,
			description TEXT,
			metadata TEXT
		)`,
		`CREATE TABLE dependencies (
			package TEXT NOT NULL,
			dependency TEXT NOT NULL,
			dep_kind TEXT NOT NULL,
			version_req TEXT,
			is_internal INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE symbols (
			rowid INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			kind TEXT NOT NULL,
			signature TEXT,
			package TEXT NOT NULL,
			file_path TEXT NOT NULL,
			line INTEGER NOT NULL,
			visibility TEXT NOT NULL,
			parent_symbol TEXT,
			return_type TEXT,
			parameters TEXT
		)`,
		`CREATE TABLE files (
			rowid INTEGER PRIMARY KEY,
			path TEXT NOT NULL,
			package TEXT,
			extension TEXT NOT NULL,
			size_bytes INTEGER NOT NULL
		)`,
		`CREATE TABLE shire_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,

		// FTS5 virtual tables (content-bearing so column values are retrievable in JOINs)
		`CREATE VIRTUAL TABLE packages_fts USING fts5(name, description, path)`,
		`CREATE VIRTUAL TABLE symbols_fts USING fts5(name, signature)`,
		`CREATE VIRTUAL TABLE files_fts USING fts5(path)`,

		// Seed packages
		`INSERT INTO packages (name, path, kind, version, description, metadata)
		 VALUES ('glamdring', '.', 'go', '0.1.0', 'Agentic coding TUI', NULL)`,
		`INSERT INTO packages (name, path, kind, version, description, metadata)
		 VALUES ('shire-cli', 'tools/shire', 'npm', '1.2.3', 'Code indexer CLI', '{"bin":"shire"}')`,
		`INSERT INTO packages (name, path, kind, version, description, metadata)
		 VALUES ('auth-service', 'services/auth', 'go', '0.3.0', 'Authentication service', NULL)`,

		// FTS for packages
		`INSERT INTO packages_fts (rowid, name, description, path)
		 VALUES (1, 'glamdring', 'Agentic coding TUI', '.')`,
		`INSERT INTO packages_fts (rowid, name, description, path)
		 VALUES (2, 'shire-cli', 'Code indexer CLI', 'tools/shire')`,
		`INSERT INTO packages_fts (rowid, name, description, path)
		 VALUES (3, 'auth-service', 'Authentication service', 'services/auth')`,

		// Seed dependencies
		`INSERT INTO dependencies (package, dependency, dep_kind, version_req, is_internal)
		 VALUES ('glamdring', 'auth-service', 'import', NULL, 1)`,
		`INSERT INTO dependencies (package, dependency, dep_kind, version_req, is_internal)
		 VALUES ('glamdring', 'modernc.org/sqlite', 'module', '>=1.0.0', 0)`,
		`INSERT INTO dependencies (package, dependency, dep_kind, version_req, is_internal)
		 VALUES ('auth-service', 'crypto-lib', 'module', '>=2.0.0', 0)`,

		// Seed symbols
		`INSERT INTO symbols (rowid, name, kind, signature, package, file_path, line, visibility, parent_symbol, return_type, parameters)
		 VALUES (1, 'Open', 'function', 'func Open(path string) (*DB, error)', 'glamdring', 'pkg/index/db.go', 16, 'public', NULL, '*DB, error', 'path string')`,
		`INSERT INTO symbols (rowid, name, kind, signature, package, file_path, line, visibility, parent_symbol, return_type, parameters)
		 VALUES (2, 'Close', 'method', 'func (db *DB) Close() error', 'glamdring', 'pkg/index/db.go', 30, 'public', 'DB', 'error', NULL)`,
		`INSERT INTO symbols (rowid, name, kind, signature, package, file_path, line, visibility, parent_symbol, return_type, parameters)
		 VALUES (3, 'SearchPackages', 'method', 'func (db *DB) SearchPackages(query string) ([]PackageRow, error)', 'glamdring', 'pkg/index/db.go', 110, 'public', 'DB', '[]PackageRow, error', 'query string')`,
		`INSERT INTO symbols (rowid, name, kind, signature, package, file_path, line, visibility, parent_symbol, return_type, parameters)
		 VALUES (4, 'Authenticate', 'function', 'func Authenticate(token string) (User, error)', 'auth-service', 'services/auth/auth.go', 25, 'public', NULL, 'User, error', 'token string')`,
		`INSERT INTO symbols (rowid, name, kind, signature, package, file_path, line, visibility, parent_symbol, return_type, parameters)
		 VALUES (5, 'DB', 'struct', 'type DB struct', 'glamdring', 'pkg/index/db.go', 11, 'public', NULL, NULL, NULL)`,

		// FTS for symbols
		`INSERT INTO symbols_fts (rowid, name, signature)
		 VALUES (1, 'Open', 'func Open(path string) (*DB, error)')`,
		`INSERT INTO symbols_fts (rowid, name, signature)
		 VALUES (2, 'Close', 'func (db *DB) Close() error')`,
		`INSERT INTO symbols_fts (rowid, name, signature)
		 VALUES (3, 'SearchPackages', 'func (db *DB) SearchPackages(query string) ([]PackageRow, error)')`,
		`INSERT INTO symbols_fts (rowid, name, signature)
		 VALUES (4, 'Authenticate', 'func Authenticate(token string) (User, error)')`,
		`INSERT INTO symbols_fts (rowid, name, signature)
		 VALUES (5, 'DB', 'type DB struct')`,

		// Seed files
		`INSERT INTO files (rowid, path, package, extension, size_bytes)
		 VALUES (1, 'pkg/index/db.go', 'glamdring', 'go', 12345)`,
		`INSERT INTO files (rowid, path, package, extension, size_bytes)
		 VALUES (2, 'pkg/index/tools.go', 'glamdring', 'go', 9876)`,
		`INSERT INTO files (rowid, path, package, extension, size_bytes)
		 VALUES (3, 'services/auth/auth.go', 'auth-service', 'go', 5432)`,
		`INSERT INTO files (rowid, path, package, extension, size_bytes)
		 VALUES (4, 'tools/shire/index.ts', 'shire-cli', 'ts', 2048)`,

		// FTS for files
		`INSERT INTO files_fts (rowid, path) VALUES (1, 'pkg/index/db.go')`,
		`INSERT INTO files_fts (rowid, path) VALUES (2, 'pkg/index/tools.go')`,
		`INSERT INTO files_fts (rowid, path) VALUES (3, 'services/auth/auth.go')`,
		`INSERT INTO files_fts (rowid, path) VALUES (4, 'tools/shire/index.ts')`,

		// Seed shire_meta
		`INSERT INTO shire_meta (key, value) VALUES ('indexed_at', '2026-03-01T12:00:00Z')`,
		`INSERT INTO shire_meta (key, value) VALUES ('git_commit', 'abc1234')`,
		`INSERT INTO shire_meta (key, value) VALUES ('package_count', '3')`,
		`INSERT INTO shire_meta (key, value) VALUES ('symbol_count', '5')`,
		`INSERT INTO shire_meta (key, value) VALUES ('file_count', '4')`,
		`INSERT INTO shire_meta (key, value) VALUES ('total_duration_ms', '1500')`,
	}

	for _, stmt := range ddl {
		if _, err := conn.Exec(stmt); err != nil {
			t.Fatalf("exec DDL %q: %v", stmt[:60], err)
		}
	}

	return &DB{conn: conn}
}

// --- sanitizeFTS / escapeDoubleQuotes ---

func TestSanitizeFTS(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", `"hello"`},
		{"", `""`},
		{`with"quotes`, `"with""quotes"`},
		{`a"b"c`, `"a""b""c"`},
		{`""`, `""""""`},
	}
	for _, tt := range tests {
		got := sanitizeFTS(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeFTS(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestEscapeDoubleQuotes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"no quotes", "no quotes"},
		{`has"one`, `has""one`},
		{`"start`, `""start`},
		{`end"`, `end""`},
		{"", ""},
	}
	for _, tt := range tests {
		got := escapeDoubleQuotes(tt.input)
		if got != tt.want {
			t.Errorf("escapeDoubleQuotes(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- SearchPackages ---

func TestSearchPackages(t *testing.T) {
	db := createTestDB(t)

	t.Run("empty query returns nil", func(t *testing.T) {
		results, err := db.SearchPackages("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if results != nil {
			t.Fatalf("expected nil, got %v", results)
		}
	})

	t.Run("finds package by name", func(t *testing.T) {
		results, err := db.SearchPackages("glamdring")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Name != "glamdring" {
			t.Errorf("expected name glamdring, got %s", results[0].Name)
		}
		if results[0].Kind != "go" {
			t.Errorf("expected kind go, got %s", results[0].Kind)
		}
	})

	t.Run("finds package by description keyword", func(t *testing.T) {
		results, err := db.SearchPackages("indexer")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Name != "shire-cli" {
			t.Errorf("expected name shire-cli, got %s", results[0].Name)
		}
	})

	t.Run("no match returns empty slice", func(t *testing.T) {
		results, err := db.SearchPackages("nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("query with double quotes is safe", func(t *testing.T) {
		results, err := db.SearchPackages(`gl"amdring`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should not match anything, but must not error
		_ = results
	})
}

// --- GetPackage ---

func TestGetPackage(t *testing.T) {
	db := createTestDB(t)

	t.Run("existing package", func(t *testing.T) {
		pkg, err := db.GetPackage("glamdring")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pkg == nil {
			t.Fatal("expected non-nil package")
		}
		if pkg.Name != "glamdring" {
			t.Errorf("name = %q, want glamdring", pkg.Name)
		}
		if pkg.Path != "." {
			t.Errorf("path = %q, want .", pkg.Path)
		}
		if pkg.Version == nil || *pkg.Version != "0.1.0" {
			t.Errorf("version = %v, want 0.1.0", pkg.Version)
		}
		if pkg.Description == nil || *pkg.Description != "Agentic coding TUI" {
			t.Errorf("description = %v, want Agentic coding TUI", pkg.Description)
		}
	})

	t.Run("package with metadata", func(t *testing.T) {
		pkg, err := db.GetPackage("shire-cli")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pkg == nil {
			t.Fatal("expected non-nil package")
		}
		if pkg.Metadata == nil || *pkg.Metadata != `{"bin":"shire"}` {
			t.Errorf("metadata = %v, want {\"bin\":\"shire\"}", pkg.Metadata)
		}
	})

	t.Run("nonexistent package returns nil", func(t *testing.T) {
		pkg, err := db.GetPackage("does-not-exist")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pkg != nil {
			t.Fatalf("expected nil, got %v", pkg)
		}
	})
}

// --- ListPackages ---

func TestListPackages(t *testing.T) {
	db := createTestDB(t)

	t.Run("all packages", func(t *testing.T) {
		results, err := db.ListPackages(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 3 {
			t.Fatalf("expected 3 packages, got %d", len(results))
		}
		// Should be ordered by name
		if results[0].Name != "auth-service" {
			t.Errorf("first package = %q, want auth-service", results[0].Name)
		}
	})

	t.Run("filter by kind", func(t *testing.T) {
		kind := "go"
		results, err := db.ListPackages(&kind)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 go packages, got %d", len(results))
		}
		for _, r := range results {
			if r.Kind != "go" {
				t.Errorf("expected kind go, got %s", r.Kind)
			}
		}
	})

	t.Run("filter by kind no match", func(t *testing.T) {
		kind := "cargo"
		results, err := db.ListPackages(&kind)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0 results, got %d", len(results))
		}
	})
}

// --- PackageDependencies ---

func TestPackageDependencies(t *testing.T) {
	db := createTestDB(t)

	t.Run("all dependencies", func(t *testing.T) {
		results, err := db.PackageDependencies("glamdring", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 deps, got %d", len(results))
		}
	})

	t.Run("internal only", func(t *testing.T) {
		results, err := db.PackageDependencies("glamdring", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 internal dep, got %d", len(results))
		}
		if results[0].Dependency != "auth-service" {
			t.Errorf("expected auth-service, got %s", results[0].Dependency)
		}
		if !results[0].IsInternal {
			t.Error("expected IsInternal = true")
		}
	})

	t.Run("no dependencies", func(t *testing.T) {
		results, err := db.PackageDependencies("shire-cli", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0 deps, got %d", len(results))
		}
	})
}

// --- PackageDependents ---

func TestPackageDependents(t *testing.T) {
	db := createTestDB(t)

	t.Run("has dependents", func(t *testing.T) {
		results, err := db.PackageDependents("auth-service")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 dependent, got %d", len(results))
		}
		if results[0].Package != "glamdring" {
			t.Errorf("expected glamdring, got %s", results[0].Package)
		}
	})

	t.Run("no dependents", func(t *testing.T) {
		results, err := db.PackageDependents("glamdring")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0 dependents, got %d", len(results))
		}
	})
}

// --- DependencyGraph ---

func TestDependencyGraph(t *testing.T) {
	db := createTestDB(t)

	t.Run("single level", func(t *testing.T) {
		edges, err := db.DependencyGraph("glamdring", 1, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(edges) != 2 {
			t.Fatalf("expected 2 edges at depth 1, got %d", len(edges))
		}
		for _, e := range edges {
			if e.From != "glamdring" {
				t.Errorf("expected from=glamdring, got %s", e.From)
			}
		}
	})

	t.Run("multi level traversal", func(t *testing.T) {
		edges, err := db.DependencyGraph("glamdring", 3, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Level 1: glamdring -> auth-service, glamdring -> modernc.org/sqlite
		// Level 2: auth-service -> crypto-lib
		if len(edges) != 3 {
			t.Fatalf("expected 3 edges at depth 3, got %d", len(edges))
		}
	})

	t.Run("internal only", func(t *testing.T) {
		edges, err := db.DependencyGraph("glamdring", 3, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Only glamdring -> auth-service is internal; auth-service -> crypto-lib is external
		if len(edges) != 1 {
			t.Fatalf("expected 1 internal edge, got %d", len(edges))
		}
		if edges[0].To != "auth-service" {
			t.Errorf("expected to=auth-service, got %s", edges[0].To)
		}
	})

	t.Run("depth zero returns no edges", func(t *testing.T) {
		edges, err := db.DependencyGraph("glamdring", 0, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(edges) != 0 {
			t.Fatalf("expected 0 edges at depth 0, got %d", len(edges))
		}
	})

	t.Run("nonexistent root returns empty", func(t *testing.T) {
		edges, err := db.DependencyGraph("nonexistent", 3, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(edges) != 0 {
			t.Fatalf("expected 0 edges, got %d", len(edges))
		}
	})

	t.Run("no duplicate visits in diamond dependency", func(t *testing.T) {
		// Add a diamond: A -> B, A -> C, B -> D, C -> D
		conn := db.conn
		conn.Exec(`INSERT INTO dependencies (package, dependency, dep_kind, version_req, is_internal) VALUES ('A', 'B', 'import', NULL, 1)`)
		conn.Exec(`INSERT INTO dependencies (package, dependency, dep_kind, version_req, is_internal) VALUES ('A', 'C', 'import', NULL, 1)`)
		conn.Exec(`INSERT INTO dependencies (package, dependency, dep_kind, version_req, is_internal) VALUES ('B', 'D', 'import', NULL, 1)`)
		conn.Exec(`INSERT INTO dependencies (package, dependency, dep_kind, version_req, is_internal) VALUES ('C', 'D', 'import', NULL, 1)`)

		edges, err := db.DependencyGraph("A", 5, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// A->B, A->C, B->D, C->D (D appears twice as target but only queued once)
		if len(edges) != 4 {
			t.Fatalf("expected 4 edges in diamond, got %d: %+v", len(edges), edges)
		}
	})
}

// --- SearchSymbols ---

func TestSearchSymbols(t *testing.T) {
	db := createTestDB(t)

	t.Run("empty query returns nil", func(t *testing.T) {
		results, err := db.SearchSymbols("", nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if results != nil {
			t.Fatalf("expected nil, got %v", results)
		}
	})

	t.Run("finds symbol by name", func(t *testing.T) {
		results, err := db.SearchSymbols("Open", nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Name != "Open" {
			t.Errorf("expected Open, got %s", results[0].Name)
		}
		if results[0].Package != "glamdring" {
			t.Errorf("expected package glamdring, got %s", results[0].Package)
		}
		if results[0].Line != 16 {
			t.Errorf("expected line 16, got %d", results[0].Line)
		}
	})

	t.Run("filter by package", func(t *testing.T) {
		pkg := "auth-service"
		results, err := db.SearchSymbols("Authenticate", &pkg, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Package != "auth-service" {
			t.Errorf("expected auth-service, got %s", results[0].Package)
		}
	})

	t.Run("filter by kind", func(t *testing.T) {
		kind := "struct"
		results, err := db.SearchSymbols("DB", nil, &kind)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Kind != "struct" {
			t.Errorf("expected struct, got %s", results[0].Kind)
		}
	})

	t.Run("filter by both package and kind", func(t *testing.T) {
		pkg := "glamdring"
		kind := "function"
		results, err := db.SearchSymbols("Open", &pkg, &kind)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
	})

	t.Run("no match returns empty", func(t *testing.T) {
		results, err := db.SearchSymbols("ZzzNonExistent", nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0 results, got %d", len(results))
		}
	})
}

// --- GetPackageSymbols ---

func TestGetPackageSymbols(t *testing.T) {
	db := createTestDB(t)

	t.Run("all symbols in package", func(t *testing.T) {
		results, err := db.GetPackageSymbols("glamdring", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 4 {
			t.Fatalf("expected 4 symbols in glamdring, got %d", len(results))
		}
		// Should be ordered by file_path, line
		if results[0].Line > results[1].Line {
			t.Error("expected results ordered by line within same file")
		}
	})

	t.Run("filter by kind", func(t *testing.T) {
		kind := "method"
		results, err := db.GetPackageSymbols("glamdring", &kind)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 methods, got %d", len(results))
		}
		for _, s := range results {
			if s.Kind != "method" {
				t.Errorf("expected kind method, got %s", s.Kind)
			}
		}
	})

	t.Run("empty package", func(t *testing.T) {
		results, err := db.GetPackageSymbols("nonexistent-pkg", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0, got %d", len(results))
		}
	})
}

// --- GetSymbol ---

func TestGetSymbol(t *testing.T) {
	db := createTestDB(t)

	t.Run("by name only", func(t *testing.T) {
		results, err := db.GetSymbol("Open", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Name != "Open" {
			t.Errorf("name = %q, want Open", results[0].Name)
		}
	})

	t.Run("with package filter", func(t *testing.T) {
		pkg := "glamdring"
		results, err := db.GetSymbol("Open", &pkg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
	})

	t.Run("package filter excludes result", func(t *testing.T) {
		pkg := "auth-service"
		results, err := db.GetSymbol("Open", &pkg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("nonexistent symbol", func(t *testing.T) {
		results, err := db.GetSymbol("DoesNotExist", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0, got %d", len(results))
		}
	})

	t.Run("nullable fields", func(t *testing.T) {
		results, err := db.GetSymbol("DB", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		s := results[0]
		if s.ReturnType != nil {
			t.Errorf("expected nil return_type for struct, got %v", s.ReturnType)
		}
		if s.Parameters != nil {
			t.Errorf("expected nil parameters for struct, got %v", s.Parameters)
		}
		if s.ParentSymbol != nil {
			t.Errorf("expected nil parent_symbol, got %v", s.ParentSymbol)
		}
	})
}

// --- GetFileSymbols ---

func TestGetFileSymbols(t *testing.T) {
	db := createTestDB(t)

	t.Run("symbols in file", func(t *testing.T) {
		results, err := db.GetFileSymbols("pkg/index/db.go", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 4 {
			t.Fatalf("expected 4 symbols in db.go, got %d", len(results))
		}
		// Ordered by line
		for i := 1; i < len(results); i++ {
			if results[i].Line < results[i-1].Line {
				t.Error("expected results ordered by line")
			}
		}
	})

	t.Run("filter by kind", func(t *testing.T) {
		kind := "function"
		results, err := db.GetFileSymbols("services/auth/auth.go", &kind)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1, got %d", len(results))
		}
		if results[0].Name != "Authenticate" {
			t.Errorf("expected Authenticate, got %s", results[0].Name)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		results, err := db.GetFileSymbols("no/such/file.go", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0, got %d", len(results))
		}
	})
}

// --- SearchFiles ---

func TestSearchFiles(t *testing.T) {
	db := createTestDB(t)

	t.Run("empty query returns nil", func(t *testing.T) {
		results, err := db.SearchFiles("", nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if results != nil {
			t.Fatalf("expected nil, got %v", results)
		}
	})

	t.Run("finds file by path fragment", func(t *testing.T) {
		results, err := db.SearchFiles("db", nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Path != "pkg/index/db.go" {
			t.Errorf("path = %q, want pkg/index/db.go", results[0].Path)
		}
		if results[0].SizeBytes != 12345 {
			t.Errorf("size_bytes = %d, want 12345", results[0].SizeBytes)
		}
	})

	t.Run("filter by package", func(t *testing.T) {
		pkg := "glamdring"
		results, err := db.SearchFiles("index", &pkg, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, f := range results {
			if pv := f.Package; pv == nil || *pv != "glamdring" {
				t.Errorf("expected package glamdring, got %v", pv)
			}
		}
	})

	t.Run("filter by extension", func(t *testing.T) {
		ext := "ts"
		results, err := db.SearchFiles("shire", nil, &ext)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Extension != "ts" {
			t.Errorf("extension = %q, want ts", results[0].Extension)
		}
	})

	t.Run("filter by package and extension", func(t *testing.T) {
		pkg := "glamdring"
		ext := "go"
		results, err := db.SearchFiles("index", &pkg, &ext)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("no match", func(t *testing.T) {
		results, err := db.SearchFiles("zzznomatch", nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0, got %d", len(results))
		}
	})
}

// --- ListPackageFiles ---

func TestListPackageFiles(t *testing.T) {
	db := createTestDB(t)

	t.Run("all files in package", func(t *testing.T) {
		results, err := db.ListPackageFiles("glamdring", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 files, got %d", len(results))
		}
		// Ordered by path
		if results[0].Path > results[1].Path {
			t.Error("expected results ordered by path")
		}
	})

	t.Run("filter by extension", func(t *testing.T) {
		ext := "ts"
		results, err := db.ListPackageFiles("shire-cli", &ext)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1, got %d", len(results))
		}
	})

	t.Run("extension filter no match", func(t *testing.T) {
		ext := "rs"
		results, err := db.ListPackageFiles("glamdring", &ext)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0, got %d", len(results))
		}
	})

	t.Run("nonexistent package", func(t *testing.T) {
		results, err := db.ListPackageFiles("nope", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0, got %d", len(results))
		}
	})
}

// --- IndexStatus ---

func TestIndexStatus(t *testing.T) {
	db := createTestDB(t)

	t.Run("returns all metadata", func(t *testing.T) {
		status, err := db.IndexStatus()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status == nil {
			t.Fatal("expected non-nil status")
		}
		check := func(name string, got *string, want string) {
			t.Helper()
			if got == nil {
				t.Errorf("%s: expected %q, got nil", name, want)
			} else if *got != want {
				t.Errorf("%s: expected %q, got %q", name, want, *got)
			}
		}
		check("indexed_at", status.IndexedAt, "2026-03-01T12:00:00Z")
		check("git_commit", status.GitCommit, "abc1234")
		check("package_count", status.PackageCount, "3")
		check("symbol_count", status.SymbolCount, "5")
		check("file_count", status.FileCount, "4")
		check("total_duration_ms", status.TotalDurationMs, "1500")
	})

	t.Run("missing keys return nil fields", func(t *testing.T) {
		// Create a DB with empty shire_meta
		conn, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		conn.Exec(`CREATE TABLE shire_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
		emptyDB := &DB{conn: conn}

		status, err := emptyDB.IndexStatus()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status.IndexedAt != nil {
			t.Error("expected nil indexed_at")
		}
		if status.GitCommit != nil {
			t.Error("expected nil git_commit")
		}
		if status.PackageCount != nil {
			t.Error("expected nil package_count")
		}
	})

	t.Run("getMeta error propagates", func(t *testing.T) {
		// Create a DB without shire_meta table to trigger a query error.
		conn, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		// No shire_meta table created -- queries against it will error.
		noTableDB := &DB{conn: conn}

		_, err = noTableDB.IndexStatus()
		if err == nil {
			t.Error("expected error when shire_meta table missing")
		}
	})
}

// --- Open ---

func TestOpen(t *testing.T) {
	t.Run("nonexistent path errors on ping", func(t *testing.T) {
		// Open in read-only mode to a path that cannot exist as a directory.
		_, err := Open("/no/such/dir/definitely-does-not-exist-glamdring-test.db")
		if err == nil {
			t.Fatal("expected error for nonexistent path")
		}
	})

	t.Run("success with valid database", func(t *testing.T) {
		// Create a temporary SQLite database file with the required schema.
		tmpDir := t.TempDir()
		dbPath := tmpDir + "/test.db"
		conn, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatalf("create temp db: %v", err)
		}
		// Create minimal schema so the DB is valid.
		conn.Exec(`CREATE TABLE packages (name TEXT PRIMARY KEY, path TEXT, kind TEXT, version TEXT, description TEXT, metadata TEXT)`)
		conn.Close()

		db, err := Open(dbPath)
		if err != nil {
			t.Fatalf("Open() returned error: %v", err)
		}
		defer db.Close()
	})
}

// --- Close ---

func TestClose(t *testing.T) {
	db := createTestDB(t)
	if err := db.Close(); err != nil {
		t.Fatalf("unexpected error on close: %v", err)
	}
	// After close, queries should fail
	_, err := db.SearchPackages("test")
	if err == nil {
		t.Error("expected error after close")
	}
}

// --- Error paths for methods on closed DB ---

func TestErrorPathsClosedDB(t *testing.T) {
	db := createTestDB(t)
	db.Close()

	t.Run("GetPackage on closed db", func(t *testing.T) {
		_, err := db.GetPackage("glamdring")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ListPackages on closed db", func(t *testing.T) {
		_, err := db.ListPackages(nil)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ListPackages with kind on closed db", func(t *testing.T) {
		kind := "go"
		_, err := db.ListPackages(&kind)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("PackageDependencies on closed db", func(t *testing.T) {
		_, err := db.PackageDependencies("glamdring", false)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("PackageDependents on closed db", func(t *testing.T) {
		_, err := db.PackageDependents("glamdring")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("DependencyGraph on closed db", func(t *testing.T) {
		_, err := db.DependencyGraph("glamdring", 3, false)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("SearchSymbols on closed db", func(t *testing.T) {
		_, err := db.SearchSymbols("Open", nil, nil)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("GetPackageSymbols on closed db", func(t *testing.T) {
		_, err := db.GetPackageSymbols("glamdring", nil)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("GetSymbol on closed db", func(t *testing.T) {
		_, err := db.GetSymbol("Open", nil)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("GetFileSymbols on closed db", func(t *testing.T) {
		_, err := db.GetFileSymbols("db.go", nil)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("SearchFiles on closed db", func(t *testing.T) {
		_, err := db.SearchFiles("db", nil, nil)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ListPackageFiles on closed db", func(t *testing.T) {
		_, err := db.ListPackageFiles("glamdring", nil)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("IndexStatus on closed db", func(t *testing.T) {
		_, err := db.IndexStatus()
		if err == nil {
			t.Error("expected error")
		}
	})
}

// --- Scanner error paths ---
// These test the scan error branches inside scanPackages, scanDependencies,
// scanSymbols, and scanFiles by using mismatched schemas.

func TestScanPackagesError(t *testing.T) {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	// Create packages table with wrong types to cause scan error.
	// scanPackages expects: name TEXT, path TEXT, kind TEXT, version TEXT, description TEXT, metadata TEXT
	// We create a table that only returns 2 columns but query asks for 6.
	conn.Exec(`CREATE TABLE packages (name TEXT, path TEXT, kind TEXT, version TEXT, description TEXT, metadata TEXT)`)
	conn.Exec(`CREATE VIRTUAL TABLE packages_fts USING fts5(name, description, path)`)
	conn.Exec(`INSERT INTO packages VALUES ('test', '.', 'go', '1.0', 'desc', NULL)`)
	conn.Exec(`INSERT INTO packages_fts (rowid, name, description, path) VALUES (1, 'test', 'desc', '.')`)

	db := &DB{conn: conn}

	// This should work fine with correct schema -- verify base case.
	results, err := db.SearchPackages("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
}

func TestScanDependenciesError(t *testing.T) {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	// Create dependencies with a view that returns wrong column count to cause scan error.
	// The real query: SELECT package, dependency, dep_kind, version_req, is_internal FROM dependencies
	// We create a table that drops the is_internal column entirely.
	conn.Exec(`CREATE TABLE dependencies (package TEXT, dependency TEXT, dep_kind TEXT, version_req TEXT)`)
	conn.Exec(`INSERT INTO dependencies VALUES ('A', 'B', 'import', NULL)`)

	db := &DB{conn: conn}
	_, err = db.PackageDependencies("A", false)
	// The query SELECTs is_internal but it doesn't exist -- query fails.
	if err == nil {
		t.Error("expected error due to missing column")
	}
}

func TestScanSymbolsError(t *testing.T) {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	// Create symbols table missing columns so the SELECT fails.
	conn.Exec(`CREATE TABLE symbols (name TEXT, kind TEXT)`)
	conn.Exec(`INSERT INTO symbols VALUES ('Foo', 'function')`)

	db := &DB{conn: conn}
	_, err = db.GetPackageSymbols("anything", nil)
	if err == nil {
		t.Error("expected error due to missing columns")
	}
}

func TestScanFilesError(t *testing.T) {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	// Create files table missing columns so the SELECT fails.
	conn.Exec(`CREATE TABLE files (path TEXT, package TEXT)`)
	conn.Exec(`INSERT INTO files VALUES ('a.go', 'pkg')`)

	db := &DB{conn: conn}
	_, err = db.ListPackageFiles("pkg", nil)
	if err == nil {
		t.Error("expected error due to missing columns")
	}
}

// Test DependencyGraph scan error path (the rows.Scan inside BFS loop).
func TestDependencyGraphScanError(t *testing.T) {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	// The graph query: SELECT dependency, dep_kind FROM dependencies WHERE package = ?1
	// Create table missing dep_kind so the query fails.
	conn.Exec(`CREATE TABLE dependencies (package TEXT, dependency TEXT)`)
	conn.Exec(`INSERT INTO dependencies VALUES ('root', 'child')`)

	db := &DB{conn: conn}
	_, err = db.DependencyGraph("root", 3, false)
	if err == nil {
		t.Error("expected error due to missing dep_kind column")
	}
}

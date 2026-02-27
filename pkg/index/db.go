package index

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB wraps a read-only connection to a shire index database.
type DB struct {
	conn *sql.DB
}

// Open opens an existing shire index database in read-only mode.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path+"?mode=ro&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open index db: %w", err)
	}
	// Verify we can actually query it.
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping index db: %w", err)
	}
	return &DB{conn: conn}, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// PackageRow represents a package in the index.
type PackageRow struct {
	Name        string  `json:"name"`
	Path        string  `json:"path"`
	Kind        string  `json:"kind"`
	Version     *string `json:"version,omitempty"`
	Description *string `json:"description,omitempty"`
	Metadata    *string `json:"metadata,omitempty"`
}

// DependencyRow represents a dependency edge.
type DependencyRow struct {
	Package    string  `json:"package"`
	Dependency string  `json:"dependency"`
	DepKind    string  `json:"dep_kind"`
	VersionReq *string `json:"version_req,omitempty"`
	IsInternal bool    `json:"is_internal"`
}

// GraphEdge represents an edge in a dependency graph traversal.
type GraphEdge struct {
	From    string `json:"from"`
	To      string `json:"to"`
	DepKind string `json:"dep_kind"`
}

// SymbolRow represents an extracted code symbol.
type SymbolRow struct {
	Name         string  `json:"name"`
	Kind         string  `json:"kind"`
	Signature    *string `json:"signature,omitempty"`
	Package      string  `json:"package"`
	FilePath     string  `json:"file_path"`
	Line         int64   `json:"line"`
	Visibility   string  `json:"visibility"`
	ParentSymbol *string `json:"parent_symbol,omitempty"`
	ReturnType   *string `json:"return_type,omitempty"`
	Parameters   *string `json:"parameters,omitempty"`
}

// FileRow represents an indexed file.
type FileRow struct {
	Path      string  `json:"path"`
	Package   *string `json:"package,omitempty"`
	Extension string  `json:"extension"`
	SizeBytes int64   `json:"size_bytes"`
}

// IndexStatus holds build metadata from the shire_meta table.
type IndexStatus struct {
	IndexedAt      *string `json:"indexed_at,omitempty"`
	GitCommit      *string `json:"git_commit,omitempty"`
	PackageCount   *string `json:"package_count,omitempty"`
	SymbolCount    *string `json:"symbol_count,omitempty"`
	FileCount      *string `json:"file_count,omitempty"`
	TotalDurationMs *string `json:"total_duration_ms,omitempty"`
}

// sanitizeFTS wraps a query in double quotes for FTS5 safety.
func sanitizeFTS(query string) string {
	return `"` + escapeDoubleQuotes(query) + `"`
}

func escapeDoubleQuotes(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			result = append(result, '"', '"')
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}

// SearchPackages performs FTS5 search across package names, descriptions, and paths.
func (db *DB) SearchPackages(query string) ([]PackageRow, error) {
	if len(query) == 0 {
		return nil, nil
	}
	sanitized := sanitizeFTS(query)
	rows, err := db.conn.Query(
		`SELECT p.name, p.path, p.kind, p.version, p.description, p.metadata
		 FROM packages_fts f
		 JOIN packages p ON p.name = f.name
		 WHERE packages_fts MATCH ?1
		 LIMIT 20`, sanitized)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPackages(rows)
}

// GetPackage retrieves a single package by exact name.
func (db *DB) GetPackage(name string) (*PackageRow, error) {
	row := db.conn.QueryRow(
		`SELECT name, path, kind, version, description, metadata
		 FROM packages WHERE name = ?1`, name)
	var p PackageRow
	err := row.Scan(&p.Name, &p.Path, &p.Kind, &p.Version, &p.Description, &p.Metadata)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListPackages returns all packages, optionally filtered by kind.
func (db *DB) ListPackages(kind *string) ([]PackageRow, error) {
	var rows *sql.Rows
	var err error
	if kind != nil {
		rows, err = db.conn.Query(
			`SELECT name, path, kind, version, description, metadata
			 FROM packages WHERE kind = ?1 ORDER BY name`, *kind)
	} else {
		rows, err = db.conn.Query(
			`SELECT name, path, kind, version, description, metadata
			 FROM packages ORDER BY name`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPackages(rows)
}

// PackageDependencies returns what a package depends on.
func (db *DB) PackageDependencies(name string, internalOnly bool) ([]DependencyRow, error) {
	q := `SELECT package, dependency, dep_kind, version_req, is_internal
	      FROM dependencies WHERE package = ?1`
	if internalOnly {
		q += ` AND is_internal = 1`
	}
	rows, err := db.conn.Query(q, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDependencies(rows)
}

// PackageDependents returns what depends on a given package (reverse lookup).
func (db *DB) PackageDependents(name string) ([]DependencyRow, error) {
	rows, err := db.conn.Query(
		`SELECT package, dependency, dep_kind, version_req, is_internal
		 FROM dependencies WHERE dependency = ?1`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDependencies(rows)
}

// DependencyGraph performs BFS traversal from root up to maxDepth levels.
func (db *DB) DependencyGraph(root string, maxDepth int, internalOnly bool) ([]GraphEdge, error) {
	q := `SELECT dependency, dep_kind FROM dependencies WHERE package = ?1`
	if internalOnly {
		q += ` AND is_internal = 1`
	}

	var edges []GraphEdge
	visited := map[string]bool{root: true}
	type item struct {
		name  string
		depth int
	}
	queue := []item{{root, 0}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth >= maxDepth {
			continue
		}
		rows, err := db.conn.Query(q, cur.name)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var dep, kind string
			if err := rows.Scan(&dep, &kind); err != nil {
				rows.Close()
				return nil, err
			}
			edges = append(edges, GraphEdge{From: cur.name, To: dep, DepKind: kind})
			if !visited[dep] {
				visited[dep] = true
				queue = append(queue, item{dep, cur.depth + 1})
			}
		}
		rows.Close()
	}
	return edges, nil
}

// SearchSymbols performs FTS5 search across symbol names and signatures.
func (db *DB) SearchSymbols(query string, packageFilter, kindFilter *string) ([]SymbolRow, error) {
	if len(query) == 0 {
		return nil, nil
	}
	sanitized := sanitizeFTS(query)

	q := `SELECT s.name, s.kind, s.signature, s.package, s.file_path, s.line,
	             s.visibility, s.parent_symbol, s.return_type, s.parameters
	      FROM symbols_fts f
	      JOIN symbols s ON s.rowid = f.rowid
	      WHERE symbols_fts MATCH ?1`
	args := []any{sanitized}
	idx := 2

	if packageFilter != nil {
		q += fmt.Sprintf(` AND s.package = ?%d`, idx)
		args = append(args, *packageFilter)
		idx++
	}
	if kindFilter != nil {
		q += fmt.Sprintf(` AND s.kind = ?%d`, idx)
		args = append(args, *kindFilter)
	}
	q += ` LIMIT 50`

	rows, err := db.conn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSymbols(rows)
}

// GetPackageSymbols lists all symbols in a package, optionally filtered by kind.
func (db *DB) GetPackageSymbols(pkg string, kindFilter *string) ([]SymbolRow, error) {
	q := `SELECT name, kind, signature, package, file_path, line,
	             visibility, parent_symbol, return_type, parameters
	      FROM symbols WHERE package = ?1`
	args := []any{pkg}
	if kindFilter != nil {
		q += ` AND kind = ?2`
		args = append(args, *kindFilter)
	}
	q += ` ORDER BY file_path, line`

	rows, err := db.conn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSymbols(rows)
}

// GetSymbol looks up symbols by exact name, optionally scoped to a package.
func (db *DB) GetSymbol(name string, packageFilter *string) ([]SymbolRow, error) {
	q := `SELECT name, kind, signature, package, file_path, line,
	             visibility, parent_symbol, return_type, parameters
	      FROM symbols WHERE name = ?1`
	args := []any{name}
	if packageFilter != nil {
		q += ` AND package = ?2`
		args = append(args, *packageFilter)
	}

	rows, err := db.conn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSymbols(rows)
}

// GetFileSymbols lists all symbols defined in a specific file.
func (db *DB) GetFileSymbols(filePath string, kindFilter *string) ([]SymbolRow, error) {
	q := `SELECT name, kind, signature, package, file_path, line,
	             visibility, parent_symbol, return_type, parameters
	      FROM symbols WHERE file_path = ?1`
	args := []any{filePath}
	if kindFilter != nil {
		q += ` AND kind = ?2`
		args = append(args, *kindFilter)
	}
	q += ` ORDER BY line`

	rows, err := db.conn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSymbols(rows)
}

// SearchFiles performs FTS5 search across file paths.
func (db *DB) SearchFiles(query string, packageFilter, extensionFilter *string) ([]FileRow, error) {
	if len(query) == 0 {
		return nil, nil
	}
	sanitized := sanitizeFTS(query)

	q := `SELECT f.path, f.package, f.extension, f.size_bytes
	      FROM files_fts fts
	      JOIN files f ON f.rowid = fts.rowid
	      WHERE files_fts MATCH ?1`
	args := []any{sanitized}
	idx := 2

	if packageFilter != nil {
		q += fmt.Sprintf(` AND f.package = ?%d`, idx)
		args = append(args, *packageFilter)
		idx++
	}
	if extensionFilter != nil {
		q += fmt.Sprintf(` AND f.extension = ?%d`, idx)
		args = append(args, *extensionFilter)
	}
	q += ` LIMIT 50`

	rows, err := db.conn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFiles(rows)
}

// ListPackageFiles lists all files belonging to a package.
func (db *DB) ListPackageFiles(pkg string, extensionFilter *string) ([]FileRow, error) {
	q := `SELECT path, package, extension, size_bytes
	      FROM files WHERE package = ?1`
	args := []any{pkg}
	if extensionFilter != nil {
		q += ` AND extension = ?2`
		args = append(args, *extensionFilter)
	}
	q += ` ORDER BY path`

	rows, err := db.conn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFiles(rows)
}

// IndexStatus reads build metadata from the shire_meta table.
func (db *DB) IndexStatus() (*IndexStatus, error) {
	getMeta := func(key string) (*string, error) {
		var val string
		err := db.conn.QueryRow(`SELECT value FROM shire_meta WHERE key = ?1`, key).Scan(&val)
		if err == sql.ErrNoRows {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &val, nil
	}

	var s IndexStatus
	var err error
	if s.IndexedAt, err = getMeta("indexed_at"); err != nil {
		return nil, err
	}
	if s.GitCommit, err = getMeta("git_commit"); err != nil {
		return nil, err
	}
	if s.PackageCount, err = getMeta("package_count"); err != nil {
		return nil, err
	}
	if s.SymbolCount, err = getMeta("symbol_count"); err != nil {
		return nil, err
	}
	if s.FileCount, err = getMeta("file_count"); err != nil {
		return nil, err
	}
	if s.TotalDurationMs, err = getMeta("total_duration_ms"); err != nil {
		return nil, err
	}
	return &s, nil
}

// --- scanners ---

func scanPackages(rows *sql.Rows) ([]PackageRow, error) {
	var result []PackageRow
	for rows.Next() {
		var p PackageRow
		if err := rows.Scan(&p.Name, &p.Path, &p.Kind, &p.Version, &p.Description, &p.Metadata); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func scanDependencies(rows *sql.Rows) ([]DependencyRow, error) {
	var result []DependencyRow
	for rows.Next() {
		var d DependencyRow
		var internal int
		if err := rows.Scan(&d.Package, &d.Dependency, &d.DepKind, &d.VersionReq, &internal); err != nil {
			return nil, err
		}
		d.IsInternal = internal != 0
		result = append(result, d)
	}
	return result, rows.Err()
}

func scanSymbols(rows *sql.Rows) ([]SymbolRow, error) {
	var result []SymbolRow
	for rows.Next() {
		var s SymbolRow
		if err := rows.Scan(&s.Name, &s.Kind, &s.Signature, &s.Package, &s.FilePath,
			&s.Line, &s.Visibility, &s.ParentSymbol, &s.ReturnType, &s.Parameters); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func scanFiles(rows *sql.Rows) ([]FileRow, error) {
	var result []FileRow
	for rows.Next() {
		var f FileRow
		if err := rows.Scan(&f.Path, &f.Package, &f.Extension, &f.SizeBytes); err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, rows.Err()
}

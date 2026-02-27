package index

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/justin/glamdring/pkg/tools"
)

// Tools returns all index tools backed by the given database.
func Tools(db *DB) []tools.Tool {
	return []tools.Tool{
		&searchPackagesTool{db},
		&getPackageTool{db},
		&listPackagesTool{db},
		&packageDependenciesTool{db},
		&packageDependentsTool{db},
		&dependencyGraphTool{db},
		&searchSymbolsTool{db},
		&getPackageSymbolsTool{db},
		&getSymbolTool{db},
		&getFileSymbolsTool{db},
		&searchFilesTool{db},
		&listPackageFilesTool{db},
		&indexStatusTool{db},
	}
}

func jsonResult(v any) (tools.Result, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("json error: %s", err), IsError: true}, nil
	}
	return tools.Result{Output: string(b)}, nil
}

func errResult(msg string) tools.Result {
	return tools.Result{Output: msg, IsError: true}
}

// --- search_packages ---

type searchPackagesTool struct{ db *DB }

func (t *searchPackagesTool) Name() string        { return "search_packages" }
func (t *searchPackagesTool) Description() string {
	return "Search packages by name or description using full-text search"
}
func (t *searchPackagesTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["query"],
		"properties": {
			"query": {"type": "string", "description": "Search query to find packages by name or description"}
		}
	}`)
}
func (t *searchPackagesTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in struct{ Query string `json:"query"` }
	if err := json.Unmarshal(input, &in); err != nil {
		return errResult(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.Query == "" {
		return errResult("search query must not be empty"), nil
	}
	results, err := t.db.SearchPackages(in.Query)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(results)
}

// --- get_package ---

type getPackageTool struct{ db *DB }

func (t *getPackageTool) Name() string        { return "get_package" }
func (t *getPackageTool) Description() string {
	return "Get full details for a specific package by exact name"
}
func (t *getPackageTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["name"],
		"properties": {
			"name": {"type": "string", "description": "Exact package name"}
		}
	}`)
}
func (t *getPackageTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in struct{ Name string `json:"name"` }
	if err := json.Unmarshal(input, &in); err != nil {
		return errResult(fmt.Sprintf("invalid input: %s", err)), nil
	}
	pkg, err := t.db.GetPackage(in.Name)
	if err != nil {
		return errResult(err.Error()), nil
	}
	if pkg == nil {
		return errResult(fmt.Sprintf("package %q not found", in.Name)), nil
	}
	return jsonResult(pkg)
}

// --- list_packages ---

type listPackagesTool struct{ db *DB }

func (t *listPackagesTool) Name() string        { return "list_packages" }
func (t *listPackagesTool) Description() string {
	return "List all indexed packages, optionally filtered by kind (npm, go, cargo, python)"
}
func (t *listPackagesTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"kind": {"type": "string", "description": "Filter by package kind: npm, go, cargo, python, maven, gradle, perl, ruby"}
		}
	}`)
}
func (t *listPackagesTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in struct{ Kind *string `json:"kind"` }
	if err := json.Unmarshal(input, &in); err != nil {
		return errResult(fmt.Sprintf("invalid input: %s", err)), nil
	}
	results, err := t.db.ListPackages(in.Kind)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(results)
}

// --- package_dependencies ---

type packageDependenciesTool struct{ db *DB }

func (t *packageDependenciesTool) Name() string { return "package_dependencies" }
func (t *packageDependenciesTool) Description() string {
	return "List what a package depends on. Set internal_only=true to see only dependencies that are other packages in this repo."
}
func (t *packageDependenciesTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["name"],
		"properties": {
			"name": {"type": "string", "description": "Package name to look up dependencies for"},
			"internal_only": {"type": "boolean", "description": "If true, only return dependencies that are also packages in this repo"}
		}
	}`)
}
func (t *packageDependenciesTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in struct {
		Name         string `json:"name"`
		InternalOnly bool   `json:"internal_only"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return errResult(fmt.Sprintf("invalid input: %s", err)), nil
	}
	results, err := t.db.PackageDependencies(in.Name, in.InternalOnly)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(results)
}

// --- package_dependents ---

type packageDependentsTool struct{ db *DB }

func (t *packageDependentsTool) Name() string { return "package_dependents" }
func (t *packageDependentsTool) Description() string {
	return "Find all packages that depend on this package (reverse dependency lookup)"
}
func (t *packageDependentsTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["name"],
		"properties": {
			"name": {"type": "string", "description": "Package name to find dependents of"}
		}
	}`)
}
func (t *packageDependentsTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in struct{ Name string `json:"name"` }
	if err := json.Unmarshal(input, &in); err != nil {
		return errResult(fmt.Sprintf("invalid input: %s", err)), nil
	}
	results, err := t.db.PackageDependents(in.Name)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(results)
}

// --- dependency_graph ---

type dependencyGraphTool struct{ db *DB }

func (t *dependencyGraphTool) Name() string { return "dependency_graph" }
func (t *dependencyGraphTool) Description() string {
	return "Get the transitive dependency graph starting from a package. Returns a list of edges. Set internal_only=true to only follow dependencies within this repo."
}
func (t *dependencyGraphTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["name"],
		"properties": {
			"name": {"type": "string", "description": "Root package to start the graph from"},
			"depth": {"type": "integer", "description": "Maximum depth to traverse (default 3)"},
			"internal_only": {"type": "boolean", "description": "If true, only follow internal dependencies"}
		}
	}`)
}
func (t *dependencyGraphTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in struct {
		Name         string `json:"name"`
		Depth        int    `json:"depth"`
		InternalOnly bool   `json:"internal_only"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return errResult(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.Depth <= 0 {
		in.Depth = 3
	}
	if in.Depth > 20 {
		in.Depth = 20
	}
	edges, err := t.db.DependencyGraph(in.Name, in.Depth, in.InternalOnly)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(edges)
}

// --- search_symbols ---

type searchSymbolsTool struct{ db *DB }

func (t *searchSymbolsTool) Name() string { return "search_symbols" }
func (t *searchSymbolsTool) Description() string {
	return "Search symbols (functions, classes, types, etc.) by name or signature using full-text search. Returns matching symbols with file location, signature, parameters, and return type."
}
func (t *searchSymbolsTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["query"],
		"properties": {
			"query": {"type": "string", "description": "Search query to find symbols by name or signature"},
			"package": {"type": "string", "description": "Filter to symbols from a specific package"},
			"kind": {"type": "string", "description": "Filter by symbol kind: function, class, struct, interface, type, enum, trait, method, constant"}
		}
	}`)
}
func (t *searchSymbolsTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in struct {
		Query   string  `json:"query"`
		Package *string `json:"package"`
		Kind    *string `json:"kind"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return errResult(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.Query == "" {
		return errResult("search query must not be empty"), nil
	}
	results, err := t.db.SearchSymbols(in.Query, in.Package, in.Kind)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(results)
}

// --- get_package_symbols ---

type getPackageSymbolsTool struct{ db *DB }

func (t *getPackageSymbolsTool) Name() string { return "get_package_symbols" }
func (t *getPackageSymbolsTool) Description() string {
	return "List all symbols in a package. Useful for understanding a package's public API — its exported functions, classes, types, and methods."
}
func (t *getPackageSymbolsTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["package"],
		"properties": {
			"package": {"type": "string", "description": "Exact package name to get symbols for"},
			"kind": {"type": "string", "description": "Filter by symbol kind: function, class, struct, interface, type, enum, trait, method, constant"}
		}
	}`)
}
func (t *getPackageSymbolsTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in struct {
		Package string  `json:"package"`
		Kind    *string `json:"kind"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return errResult(fmt.Sprintf("invalid input: %s", err)), nil
	}
	results, err := t.db.GetPackageSymbols(in.Package, in.Kind)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(results)
}

// --- get_symbol ---

type getSymbolTool struct{ db *DB }

func (t *getSymbolTool) Name() string        { return "get_symbol" }
func (t *getSymbolTool) Description() string {
	return "Get details for a specific symbol by exact name. Returns all symbols matching that name across packages, with file location, signature, parameters, and return type."
}
func (t *getSymbolTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["name"],
		"properties": {
			"name": {"type": "string", "description": "Exact symbol name to look up"},
			"package": {"type": "string", "description": "Filter to a specific package"}
		}
	}`)
}
func (t *getSymbolTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in struct {
		Name    string  `json:"name"`
		Package *string `json:"package"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return errResult(fmt.Sprintf("invalid input: %s", err)), nil
	}
	results, err := t.db.GetSymbol(in.Name, in.Package)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(results)
}

// --- get_file_symbols ---

type getFileSymbolsTool struct{ db *DB }

func (t *getFileSymbolsTool) Name() string { return "get_file_symbols" }
func (t *getFileSymbolsTool) Description() string {
	return "List all symbols defined in a specific file. Useful for understanding what a file exports — its functions, classes, types, and methods."
}
func (t *getFileSymbolsTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["file_path"],
		"properties": {
			"file_path": {"type": "string", "description": "File path relative to repo root (e.g., services/auth/src/auth.ts)"},
			"kind": {"type": "string", "description": "Filter by symbol kind: function, class, struct, interface, type, enum, trait, method, constant"}
		}
	}`)
}
func (t *getFileSymbolsTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in struct {
		FilePath string  `json:"file_path"`
		Kind     *string `json:"kind"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return errResult(fmt.Sprintf("invalid input: %s", err)), nil
	}
	results, err := t.db.GetFileSymbols(in.FilePath, in.Kind)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(results)
}

// --- search_files ---

type searchFilesTool struct{ db *DB }

func (t *searchFilesTool) Name() string { return "search_files" }
func (t *searchFilesTool) Description() string {
	return "Search files by path or name using full-text search. Useful for finding files like 'middleware', 'proto files', or files in a specific directory."
}
func (t *searchFilesTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["query"],
		"properties": {
			"query": {"type": "string", "description": "Search query to find files by path or name"},
			"package": {"type": "string", "description": "Filter to files from a specific package"},
			"extension": {"type": "string", "description": "Filter by file extension (e.g., ts, go, rs)"}
		}
	}`)
}
func (t *searchFilesTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in struct {
		Query     string  `json:"query"`
		Package   *string `json:"package"`
		Extension *string `json:"extension"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return errResult(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.Query == "" {
		return errResult("search query must not be empty"), nil
	}
	results, err := t.db.SearchFiles(in.Query, in.Package, in.Extension)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(results)
}

// --- list_package_files ---

type listPackageFilesTool struct{ db *DB }

func (t *listPackageFilesTool) Name() string { return "list_package_files" }
func (t *listPackageFilesTool) Description() string {
	return "List all files belonging to a specific package. Optionally filter by file extension."
}
func (t *listPackageFilesTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["package"],
		"properties": {
			"package": {"type": "string", "description": "Exact package name to list files for"},
			"extension": {"type": "string", "description": "Filter by file extension (e.g., ts, go, rs)"}
		}
	}`)
}
func (t *listPackageFilesTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in struct {
		Package   string  `json:"package"`
		Extension *string `json:"extension"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return errResult(fmt.Sprintf("invalid input: %s", err)), nil
	}
	results, err := t.db.ListPackageFiles(in.Package, in.Extension)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(results)
}

// --- index_status ---

type indexStatusTool struct{ db *DB }

func (t *indexStatusTool) Name() string { return "index_status" }
func (t *indexStatusTool) Description() string {
	return "Get index status: when it was built, git commit, package/symbol/file counts, and build duration in milliseconds"
}
func (t *indexStatusTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {}
	}`)
}
func (t *indexStatusTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	status, err := t.db.IndexStatus()
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(status)
}

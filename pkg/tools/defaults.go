package tools

// DefaultTools returns all built-in tools, configured with the given working directory.
func DefaultTools(cwd string) []Tool {
	return []Tool{
		ReadTool{},
		WriteTool{},
		EditTool{},
		BashTool{CWD: cwd},
		GlobTool{CWD: cwd},
		GrepTool{CWD: cwd},
	}
}

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

// DefaultToolsWithTask returns all built-in tools plus the Task tool for
// subagent spawning. The Task tool is appended to the end of the list.
func DefaultToolsWithTask(cwd string, taskTool *TaskTool) []Tool {
	base := DefaultTools(cwd)
	if taskTool != nil {
		base = append(base, taskTool)
	}
	return base
}

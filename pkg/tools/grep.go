package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// GrepTool searches file contents using regular expressions.
type GrepTool struct {
	CWD string
}

type grepInput struct {
	Pattern         string `json:"pattern"`
	Path            string `json:"path"`
	Glob            string `json:"glob"`
	OutputMode      string `json:"output_mode"`
	Context         int    `json:"context"`
	CaseInsensitive bool   `json:"case_insensitive"`
	HeadLimit       int    `json:"head_limit"`
	Offset          int    `json:"offset"`
	Type            string `json:"type"`
}

// fileTypeMap maps short type names to glob patterns.
var fileTypeMap = map[string]string{
	"js":     "*.js",
	"ts":     "*.ts",
	"tsx":    "*.tsx",
	"jsx":    "*.jsx",
	"py":     "*.py",
	"go":     "*.go",
	"rust":   "*.rs",
	"rs":     "*.rs",
	"java":   "*.java",
	"rb":     "*.rb",
	"c":      "*.c",
	"cpp":    "*.cpp",
	"h":      "*.h",
	"css":    "*.css",
	"html":   "*.html",
	"json":   "*.json",
	"yaml":   "*.yaml",
	"yml":    "*.yml",
	"md":     "*.md",
	"sql":    "*.sql",
	"sh":     "*.sh",
	"swift":  "*.swift",
	"kt":     "*.kt",
	"scala":  "*.scala",
	"php":    "*.php",
	"toml":   "*.toml",
	"xml":    "*.xml",
	"svelte": "*.svelte",
	"vue":    "*.vue",
}

func (GrepTool) Name() string        { return "Grep" }
func (GrepTool) Description() string { return "Search file contents with regex" }

func (GrepTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["pattern"],
		"properties": {
			"pattern": {
				"type": "string",
				"description": "Regular expression pattern to search for"
			},
			"path": {
				"type": "string",
				"description": "File or directory to search in (defaults to CWD)"
			},
			"glob": {
				"type": "string",
				"description": "Glob pattern to filter files (e.g. *.go)"
			},
			"output_mode": {
				"type": "string",
				"enum": ["content", "files_with_matches", "count"],
				"description": "Output mode (default files_with_matches)"
			},
			"context": {
				"type": "integer",
				"description": "Lines of context before and after match"
			},
			"-A": {
				"type": "integer",
				"description": "Lines of context after each match"
			},
			"-B": {
				"type": "integer",
				"description": "Lines of context before each match"
			},
			"-C": {
				"type": "integer",
				"description": "Lines of context before and after each match (alias for context)"
			},
			"-i": {
				"type": "boolean",
				"description": "Case insensitive search"
			},
			"-n": {
				"type": "boolean",
				"description": "Show line numbers (default true, only for content mode)"
			},
			"head_limit": {
				"type": "integer",
				"description": "Limit output to first N entries"
			},
			"offset": {
				"type": "integer",
				"description": "Skip first N entries before applying head_limit"
			},
			"type": {
				"type": "string",
				"description": "File type to search (e.g. js, py, go, rust)"
			},
			"case_insensitive": {
				"type": "boolean",
				"description": "Case insensitive search"
			}
		}
	}`)
}

func (t GrepTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in grepInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Output: fmt.Sprintf("invalid input: %s", err), IsError: true}, nil
	}

	// Parse -A, -B, -C, -i, -n from raw JSON (these have special keys).
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(input, &raw)

	var contextA, contextB, contextC int
	showLineNumbers := true // default true

	if v, ok := raw["-A"]; ok {
		_ = json.Unmarshal(v, &contextA)
	}
	if v, ok := raw["-B"]; ok {
		_ = json.Unmarshal(v, &contextB)
	}
	if v, ok := raw["-C"]; ok {
		_ = json.Unmarshal(v, &contextC)
	}
	if v, ok := raw["-i"]; ok {
		var ci bool
		_ = json.Unmarshal(v, &ci)
		if ci {
			in.CaseInsensitive = true
		}
	}
	if v, ok := raw["-n"]; ok {
		_ = json.Unmarshal(v, &showLineNumbers)
	}

	if in.Pattern == "" {
		return Result{Output: "pattern is required", IsError: true}, nil
	}

	if in.OutputMode == "" {
		in.OutputMode = "files_with_matches"
	}

	// Resolve context lines. -C overrides context, and -A/-B override after/before.
	ctxBefore := in.Context
	ctxAfter := in.Context
	if contextC > 0 {
		ctxBefore = contextC
		ctxAfter = contextC
	}
	if contextB > 0 {
		ctxBefore = contextB
	}
	if contextA > 0 {
		ctxAfter = contextA
	}

	// Compile regex.
	patternStr := in.Pattern
	if in.CaseInsensitive {
		patternStr = "(?i)" + patternStr
	}
	re, err := regexp.Compile(patternStr)
	if err != nil {
		return Result{Output: fmt.Sprintf("invalid regex: %s", err), IsError: true}, nil
	}

	root := t.CWD
	if in.Path != "" {
		root = in.Path
	}
	if root == "" {
		root, err = os.Getwd()
		if err != nil {
			return Result{Output: fmt.Sprintf("failed to get working directory: %s", err), IsError: true}, nil
		}
	}

	// Resolve glob from type filter if no explicit glob.
	globPattern := in.Glob
	if globPattern == "" && in.Type != "" {
		if pattern, ok := fileTypeMap[in.Type]; ok {
			globPattern = pattern
		} else {
			// Treat unknown type as a glob extension.
			globPattern = "*." + in.Type
		}
	}

	// Determine if root is a file or directory.
	info, err := os.Stat(root)
	if err != nil {
		return Result{Output: fmt.Sprintf("path error: %s", err), IsError: true}, nil
	}

	var files []string
	if !info.IsDir() {
		files = []string{root}
	} else {
		files, err = collectFiles(ctx, root, globPattern)
		if err != nil {
			return Result{Output: fmt.Sprintf("file collection error: %s", err), IsError: true}, nil
		}
	}

	var buf strings.Builder
	for _, fpath := range files {
		if ctx.Err() != nil {
			break
		}
		searchFile(fpath, re, in.OutputMode, ctxBefore, ctxAfter, showLineNumbers, &buf)
	}

	if buf.Len() == 0 {
		return Result{Output: "no matches found"}, nil
	}

	output := buf.String()
	output = applyHeadLimitOffset(output, in.Offset, in.HeadLimit)

	return Result{Output: output}, nil
}

// applyHeadLimitOffset applies offset and head_limit to line-based output.
func applyHeadLimitOffset(output string, offset, limit int) string {
	if offset <= 0 && limit <= 0 {
		return output
	}

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	if offset > 0 {
		if offset >= len(lines) {
			return ""
		}
		lines = lines[offset:]
	}

	if limit > 0 && limit < len(lines) {
		lines = lines[:limit]
	}

	return strings.Join(lines, "\n") + "\n"
}

// collectFiles walks a directory and returns file paths, optionally filtered by glob.
func collectFiles(ctx context.Context, root, globPattern string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			name := d.Name()
			if name != "." && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			// Skip common noise directories.
			switch name {
			case "node_modules", "__pycache__", ".next", "dist", "build", ".venv", ".tox":
				return filepath.SkipDir
			}
			return nil
		}

		// Check for binary files: read first 8KB and look for null bytes.
		if isBinaryFile(path) {
			return nil
		}

		if globPattern != "" {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return nil
			}
			matched, matchErr := doublestar.Match(globPattern, rel)
			if matchErr != nil || !matched {
				matched, _ = doublestar.Match(globPattern, d.Name())
				if !matched {
					return nil
				}
			}
		}

		files = append(files, path)
		return nil
	})
	return files, err
}

// isBinaryFile checks if a file appears to be binary by looking for null bytes in the first 8KB.
func isBinaryFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 8192)
	n, err := f.Read(buf)
	if n == 0 {
		return false
	}
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}
	return false
}

// searchFile searches a single file and writes results to buf.
func searchFile(fpath string, re *regexp.Regexp, mode string, ctxBefore, ctxAfter int, showLineNumbers bool, buf *strings.Builder) {
	f, err := os.Open(fpath)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if scanner.Err() != nil {
		return
	}

	var matchIndices []int
	for i, line := range lines {
		if re.MatchString(line) {
			matchIndices = append(matchIndices, i)
		}
	}

	if len(matchIndices) == 0 {
		return
	}

	switch mode {
	case "files_with_matches":
		buf.WriteString(fpath)
		buf.WriteByte('\n')

	case "count":
		fmt.Fprintf(buf, "%s:%d\n", fpath, len(matchIndices))

	case "content":
		show := make(map[int]bool)
		for _, idx := range matchIndices {
			start := idx - ctxBefore
			if start < 0 {
				start = 0
			}
			end := idx + ctxAfter
			if end >= len(lines) {
				end = len(lines) - 1
			}
			for i := start; i <= end; i++ {
				show[i] = true
			}
		}

		fmt.Fprintf(buf, "%s:\n", fpath)
		prevShown := -2
		for i := 0; i < len(lines); i++ {
			if !show[i] {
				continue
			}
			if prevShown >= 0 && i > prevShown+1 {
				buf.WriteString("--\n")
			}
			if showLineNumbers {
				fmt.Fprintf(buf, "%d:%s\n", i+1, lines[i])
			} else {
				buf.WriteString(lines[i])
				buf.WriteByte('\n')
			}
			prevShown = i
		}
		buf.WriteByte('\n')
	}
}

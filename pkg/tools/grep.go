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
	ContextA        int    `json:"-A"`
	ContextB        int    `json:"-B"`
	ContextC        int    `json:"-C"`
	CaseInsensitive bool   `json:"case_insensitive"`
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
			"case_insensitive": {
				"type": "boolean",
				"description": "Case insensitive search"
			}
		}
	}`)
}

func (t GrepTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in grepInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Output: fmt.Sprintf("invalid input: %s", err), IsError: true}, nil
	}

	// Also handle -A, -B, -C from raw JSON.
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(input, &raw)
	if v, ok := raw["-A"]; ok {
		_ = json.Unmarshal(v, &in.ContextA)
	}
	if v, ok := raw["-B"]; ok {
		_ = json.Unmarshal(v, &in.ContextB)
	}
	if v, ok := raw["-C"]; ok {
		_ = json.Unmarshal(v, &in.ContextC)
	}

	if in.Pattern == "" {
		return Result{Output: "pattern is required", IsError: true}, nil
	}

	// Default output mode.
	if in.OutputMode == "" {
		in.OutputMode = "files_with_matches"
	}

	// Resolve context lines. -C overrides context, and -A/-B override after/before.
	ctxBefore := in.Context
	ctxAfter := in.Context
	if in.ContextC > 0 {
		ctxBefore = in.ContextC
		ctxAfter = in.ContextC
	}
	if in.ContextB > 0 {
		ctxBefore = in.ContextB
	}
	if in.ContextA > 0 {
		ctxAfter = in.ContextA
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

	// Determine if root is a file or directory.
	info, err := os.Stat(root)
	if err != nil {
		return Result{Output: fmt.Sprintf("path error: %s", err), IsError: true}, nil
	}

	var files []string
	if !info.IsDir() {
		files = []string{root}
	} else {
		files, err = collectFiles(root, in.Glob)
		if err != nil {
			return Result{Output: fmt.Sprintf("file collection error: %s", err), IsError: true}, nil
		}
	}

	var buf strings.Builder
	for _, fpath := range files {
		searchFile(fpath, re, in.OutputMode, ctxBefore, ctxAfter, &buf)
	}

	if buf.Len() == 0 {
		return Result{Output: "no matches found"}, nil
	}

	return Result{Output: buf.String()}, nil
}

// collectFiles walks a directory and returns file paths, optionally filtered by glob.
func collectFiles(root, globPattern string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}
		if d.IsDir() {
			// Skip hidden directories.
			if d.Name() != "." && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Apply glob filter if specified.
		if globPattern != "" {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return nil
			}
			matched, matchErr := doublestar.Match(globPattern, rel)
			if matchErr != nil || !matched {
				// Also try matching just the filename.
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

// searchFile searches a single file and writes results to buf.
func searchFile(fpath string, re *regexp.Regexp, mode string, ctxBefore, ctxAfter int, buf *strings.Builder) {
	f, err := os.Open(fpath)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 1MB line buffer

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if scanner.Err() != nil {
		return
	}

	// Find matching line indices.
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
		// Determine which lines to show (matches + context).
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

		// Build a set of match indices for highlighting.
		matchSet := make(map[int]bool)
		for _, idx := range matchIndices {
			matchSet[idx] = true
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
			fmt.Fprintf(buf, "%d:%s\n", i+1, lines[i])
			prevShown = i
		}
		buf.WriteByte('\n')
	}
}

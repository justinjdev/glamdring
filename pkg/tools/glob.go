package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// GlobTool finds files matching a glob pattern.
type GlobTool struct {
	CWD string
}

type globInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
}

func (GlobTool) Name() string        { return "Glob" }
func (GlobTool) Description() string { return "Find files matching a glob pattern" }

func (GlobTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["pattern"],
		"properties": {
			"pattern": {
				"type": "string",
				"description": "Glob pattern to match (supports **)"
			},
			"path": {
				"type": "string",
				"description": "Directory to search in (defaults to CWD)"
			}
		}
	}`)
}

type fileWithTime struct {
	path    string
	modTime int64
}

func (t GlobTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in globInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Output: fmt.Sprintf("invalid input: %s", err), IsError: true}, nil
	}
	if in.Pattern == "" {
		return Result{Output: "pattern is required", IsError: true}, nil
	}

	root := t.CWD
	if in.Path != "" {
		root = in.Path
	}
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return Result{Output: fmt.Sprintf("failed to get working directory: %s", err), IsError: true}, nil
		}
	}

	var matches []fileWithTime

	fsys := os.DirFS(root)
	err := doublestar.GlobWalk(fsys, in.Pattern, func(path string, d fs.DirEntry) error {
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil // skip files we can't stat
		}
		fullPath := filepath.Join(root, path)
		matches = append(matches, fileWithTime{path: fullPath, modTime: info.ModTime().UnixNano()})
		return nil
	})
	if err != nil {
		return Result{Output: fmt.Sprintf("glob error: %s", err), IsError: true}, nil
	}

	// Sort by modification time, most recent first.
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].modTime > matches[j].modTime
	})

	var buf strings.Builder
	for _, m := range matches {
		buf.WriteString(m.path)
		buf.WriteByte('\n')
	}

	if buf.Len() == 0 {
		return Result{Output: "no matches found"}, nil
	}

	return Result{Output: buf.String()}, nil
}

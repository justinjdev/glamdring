package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

const defaultGlobLimit = 1000

// noiseDirectories are directories skipped during glob walks unless explicitly targeted.
var noiseDirectories = map[string]bool{
	".git":         true,
	"node_modules": true,
	"__pycache__":  true,
	".next":        true,
	"dist":         true,
	"build":        true,
	".venv":        true,
	".tox":         true,
}

// GlobTool finds files matching a glob pattern.
type GlobTool struct {
	CWD string
}

type globInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
	Limit   int    `json:"limit"`
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
			},
			"limit": {
				"type": "integer",
				"description": "Maximum number of results (default 1000)"
			}
		}
	}`)
}

type fileWithTime struct {
	path    string
	modTime int64
}

func (t GlobTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
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

	limit := in.Limit
	if limit <= 0 {
		limit = defaultGlobLimit
	}

	// Check if pattern explicitly targets a noise directory.
	patternTargetsNoise := false
	for dir := range noiseDirectories {
		if strings.Contains(in.Pattern, dir+"/") || strings.HasPrefix(in.Pattern, dir) {
			patternTargetsNoise = true
			break
		}
	}

	var matches []fileWithTime
	hitLimit := false

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if d.IsDir() {
			name := d.Name()
			// Skip noise directories unless pattern explicitly targets them.
			if !patternTargetsNoise && noiseDirectories[name] && path != root {
				return filepath.SkipDir
			}
			return nil
		}

		// Match against the pattern.
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		matched, matchErr := doublestar.Match(in.Pattern, rel)
		if matchErr != nil || !matched {
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}
		matches = append(matches, fileWithTime{path: filepath.Join(root, rel), modTime: info.ModTime().UnixNano()})

		// Stop early if we've collected more than we need (with buffer for sorting).
		if len(matches) > limit*2 {
			hitLimit = true
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil && err != filepath.SkipAll {
		return Result{Output: fmt.Sprintf("glob error: %s", err), IsError: true}, nil
	}

	// Sort by modification time, most recent first.
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].modTime > matches[j].modTime
	})

	// Apply limit.
	truncated := false
	totalCount := len(matches)
	if len(matches) > limit {
		matches = matches[:limit]
		truncated = true
	}

	var buf strings.Builder
	for _, m := range matches {
		buf.WriteString(m.path)
		buf.WriteByte('\n')
	}

	if truncated || hitLimit {
		fmt.Fprintf(&buf, "\n... (showing %d of %d+ results, use a more specific pattern)\n", limit, totalCount)
	}

	if buf.Len() == 0 {
		return Result{Output: "no matches found"}, nil
	}

	return Result{Output: buf.String()}, nil
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// EditTool performs exact string replacement in a file.
type EditTool struct{}

type editInput struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all"`
}

func (EditTool) Name() string        { return "Edit" }
func (EditTool) Description() string { return "Exact string replacement in a file" }

func (EditTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["file_path", "old_string", "new_string"],
		"properties": {
			"file_path": {
				"type": "string",
				"description": "Absolute path to the file to edit"
			},
			"old_string": {
				"type": "string",
				"description": "The exact string to find and replace"
			},
			"new_string": {
				"type": "string",
				"description": "The replacement string"
			},
			"replace_all": {
				"type": "boolean",
				"description": "Replace all occurrences (default false)"
			}
		}
	}`)
}

func (EditTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in editInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Output: fmt.Sprintf("invalid input: %s", err), IsError: true}, nil
	}
	if in.FilePath == "" {
		return Result{Output: "file_path is required", IsError: true}, nil
	}

	// Reject no-op edits.
	if in.OldString == in.NewString {
		return Result{Output: "old_string and new_string are identical", IsError: true}, nil
	}

	// Stat the file to preserve permissions.
	info, err := os.Stat(in.FilePath)
	if err != nil {
		return Result{Output: fmt.Sprintf("failed to stat file: %s", err), IsError: true}, nil
	}
	fileMode := info.Mode()

	data, err := os.ReadFile(in.FilePath)
	if err != nil {
		return Result{Output: fmt.Sprintf("failed to read file: %s", err), IsError: true}, nil
	}

	content := string(data)
	count := strings.Count(content, in.OldString)

	if count == 0 {
		return Result{Output: "old_string not found in file", IsError: true}, nil
	}

	if count > 1 && !in.ReplaceAll {
		return Result{
			Output:  fmt.Sprintf("old_string found %d times; use replace_all to replace all occurrences", count),
			IsError: true,
		}, nil
	}

	var newContent string
	if in.ReplaceAll {
		newContent = strings.ReplaceAll(content, in.OldString, in.NewString)
	} else {
		newContent = strings.Replace(content, in.OldString, in.NewString, 1)
	}

	if err := os.WriteFile(in.FilePath, []byte(newContent), fileMode); err != nil {
		return Result{Output: fmt.Sprintf("failed to write file: %s", err), IsError: true}, nil
	}

	return Result{Output: fmt.Sprintf("replaced %d occurrence(s) in %s", count, in.FilePath)}, nil
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteTool creates or overwrites a file.
type WriteTool struct {
	Tracker *ReadTracker
}

type writeInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

func (WriteTool) Name() string        { return "Write" }
func (WriteTool) Description() string { return "Create or overwrite a file" }

func (WriteTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["file_path", "content"],
		"properties": {
			"file_path": {
				"type": "string",
				"description": "Absolute path to the file to write"
			},
			"content": {
				"type": "string",
				"description": "Content to write to the file"
			}
		}
	}`)
}

func (t WriteTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in writeInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Output: fmt.Sprintf("invalid input: %s", err), IsError: true}, nil
	}
	if in.FilePath == "" {
		return Result{Output: "file_path is required", IsError: true}, nil
	}

	// Safety check: require read before overwriting existing files.
	if _, err := os.Stat(in.FilePath); err == nil {
		// File exists — check if it was read first.
		if t.Tracker != nil && !t.Tracker.HasRead(in.FilePath) {
			return Result{
				Output:  "File has not been read in this session. Read it first to avoid accidental overwrites.",
				IsError: true,
			}, nil
		}
	}

	dir := filepath.Dir(in.FilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Result{Output: fmt.Sprintf("failed to create directories: %s", err), IsError: true}, nil
	}

	if err := os.WriteFile(in.FilePath, []byte(in.Content), 0o644); err != nil {
		return Result{Output: fmt.Sprintf("failed to write file: %s", err), IsError: true}, nil
	}

	return Result{Output: fmt.Sprintf("wrote %d bytes to %s", len(in.Content), in.FilePath)}, nil
}

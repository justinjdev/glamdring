package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ReadTool reads file contents with line numbers.
type ReadTool struct{}

type readInput struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

func (ReadTool) Name() string        { return "Read" }
func (ReadTool) Description() string { return "Read file contents with line numbers" }

func (ReadTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["file_path"],
		"properties": {
			"file_path": {
				"type": "string",
				"description": "Absolute path to the file to read"
			},
			"offset": {
				"type": "integer",
				"description": "Line number to start reading from (1-based)"
			},
			"limit": {
				"type": "integer",
				"description": "Number of lines to read"
			}
		}
	}`)
}

func (ReadTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in readInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Output: fmt.Sprintf("invalid input: %s", err), IsError: true}, nil
	}
	if in.FilePath == "" {
		return Result{Output: "file_path is required", IsError: true}, nil
	}

	data, err := os.ReadFile(in.FilePath)
	if err != nil {
		return Result{Output: fmt.Sprintf("failed to read file: %s", err), IsError: true}, nil
	}

	lines := strings.Split(string(data), "\n")

	// Apply offset (1-based).
	start := 0
	if in.Offset > 0 {
		start = in.Offset - 1
	}
	if start > len(lines) {
		start = len(lines)
	}

	end := len(lines)
	if in.Limit > 0 && start+in.Limit < end {
		end = start + in.Limit
	}

	var buf strings.Builder
	for i := start; i < end; i++ {
		fmt.Fprintf(&buf, "%6d\t%s\n", i+1, lines[i])
	}

	return Result{Output: buf.String()}, nil
}

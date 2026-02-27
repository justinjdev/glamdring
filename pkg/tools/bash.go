package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// BashTool executes shell commands.
type BashTool struct {
	CWD string
}

type bashInput struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"` // milliseconds
}

func (BashTool) Name() string        { return "Bash" }
func (BashTool) Description() string { return "Execute a shell command" }

func (BashTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["command"],
		"properties": {
			"command": {
				"type": "string",
				"description": "The bash command to execute"
			},
			"timeout": {
				"type": "integer",
				"description": "Timeout in milliseconds (default 120000)"
			}
		}
	}`)
}

func (t BashTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in bashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Output: fmt.Sprintf("invalid input: %s", err), IsError: true}, nil
	}
	if in.Command == "" {
		return Result{Output: "command is required", IsError: true}, nil
	}

	timeout := 120 * time.Second
	if in.Timeout > 0 {
		timeout = time.Duration(in.Timeout) * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", in.Command)
	cmd.Dir = t.CWD
	// Use process group so we can kill the entire tree on timeout.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			// Kill the process group on timeout.
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
			}
			return Result{Output: "command timed out", IsError: true}, nil
		} else {
			return Result{Output: fmt.Sprintf("failed to run command: %s", err), IsError: true}, nil
		}
	}

	var out strings.Builder
	if stdout.Len() > 0 {
		out.WriteString(stdout.String())
	}
	if stderr.Len() > 0 {
		if out.Len() > 0 {
			out.WriteString("\n")
		}
		out.WriteString("STDERR:\n")
		out.WriteString(stderr.String())
	}

	if exitCode != 0 {
		fmt.Fprintf(&out, "\nexit code: %d", exitCode)
	}

	return Result{Output: out.String(), IsError: exitCode != 0}, nil
}

package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	maxTimeout    = 600000 // milliseconds
	maxOutputSize = 1 << 20 // 1MB
	tailLines     = 500
)

// BashTool executes shell commands.
type BashTool struct {
	CWD string
}

type bashInput struct {
	Command        string `json:"command"`
	Timeout        int    `json:"timeout"`
	RunInBackground bool  `json:"run_in_background"`
}

// bgProcess tracks a background process.
type bgProcess struct {
	PID     int
	Command string
	Done    chan struct{}
	Output  string
	Err     error
}

var (
	bgMu        sync.Mutex
	bgProcesses = make(map[int]*bgProcess)
)

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
				"description": "Timeout in milliseconds (default 120000, max 600000)"
			},
			"run_in_background": {
				"type": "boolean",
				"description": "Run the command in the background and return immediately with the PID"
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
		t := in.Timeout
		if t > maxTimeout {
			t = maxTimeout
		}
		timeout = time.Duration(t) * time.Millisecond
	}

	if in.RunInBackground {
		return t.executeBackground(ctx, in.Command)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", in.Command)
	cmd.Dir = t.CWD
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Check for timeout BEFORE checking ExitError.
	if ctx.Err() == context.DeadlineExceeded {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return Result{Output: "command timed out", IsError: true}, nil
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return Result{Output: fmt.Sprintf("failed to run command: %s", err), IsError: true}, nil
		}
	}

	output := buildOutput(stdout.Bytes(), stderr.Bytes(), exitCode)
	output = truncateOutput(output)

	return Result{Output: output, IsError: exitCode != 0}, nil
}

func (t BashTool) executeBackground(_ context.Context, command string) (Result, error) {
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = t.CWD
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return Result{Output: fmt.Sprintf("failed to start background command: %s", err), IsError: true}, nil
	}

	pid := cmd.Process.Pid
	bp := &bgProcess{
		PID:     pid,
		Command: command,
		Done:    make(chan struct{}),
	}

	bgMu.Lock()
	bgProcesses[pid] = bp
	bgMu.Unlock()

	go func() {
		defer close(bp.Done)
		err := cmd.Wait()
		bp.Output = buildOutput(stdout.Bytes(), stderr.Bytes(), 0)
		if err != nil {
			bp.Err = err
		}
		bp.Output = truncateOutput(bp.Output)
	}()

	return Result{Output: fmt.Sprintf("background process started with PID %d", pid)}, nil
}

// buildOutput combines stdout and stderr into a single output string.
func buildOutput(stdout, stderr []byte, exitCode int) string {
	var out strings.Builder
	if len(stdout) > 0 {
		out.Write(stdout)
	}
	if len(stderr) > 0 {
		if out.Len() > 0 {
			out.WriteString("\n")
		}
		out.WriteString("STDERR:\n")
		out.Write(stderr)
	}
	if exitCode != 0 {
		fmt.Fprintf(&out, "\nexit code: %d", exitCode)
	}
	return out.String()
}

// truncateOutput caps output at maxOutputSize, keeping the last tailLines.
func truncateOutput(output string) string {
	if len(output) <= maxOutputSize {
		return output
	}

	lines := strings.Split(output, "\n")
	start := len(lines) - tailLines
	if start < 0 {
		start = 0
	}
	kept := lines[start:]
	return fmt.Sprintf("... (output truncated, showing last %d lines)\n%s", len(kept), strings.Join(kept, "\n"))
}

package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

// parseBashInput unmarshals and validates bash tool input.
func parseBashInput(input json.RawMessage) (bashInput, *Result) {
	var in bashInput
	if err := json.Unmarshal(input, &in); err != nil {
		r := Result{Output: fmt.Sprintf("invalid input: %s", err), IsError: true}
		return in, &r
	}
	if in.Command == "" {
		r := Result{Output: "command is required", IsError: true}
		return in, &r
	}
	return in, nil
}

func (t BashTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	in, errResult := parseBashInput(input)
	if errResult != nil {
		return *errResult, nil
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
			if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
				log.Printf("failed to kill process group %d: %v", cmd.Process.Pid, err)
			}
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

// ExecuteStreaming runs a command with real-time output streaming via onOutput.
// Background commands fall back to the non-streaming Execute path.
func (t BashTool) ExecuteStreaming(ctx context.Context, input json.RawMessage, onOutput func(string)) (Result, error) {
	in, errResult := parseBashInput(input)
	if errResult != nil {
		return *errResult, nil
	}

	// Background jobs don't stream.
	if in.RunInBackground {
		return t.executeBackground(ctx, in.Command)
	}

	timeout := 120 * time.Second
	if in.Timeout > 0 {
		tt := in.Timeout
		if tt > maxTimeout {
			tt = maxTimeout
		}
		timeout = time.Duration(tt) * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", in.Command)
	cmd.Dir = t.CWD
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return Result{Output: fmt.Sprintf("failed to create stdout pipe: %s", err), IsError: true}, nil
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return Result{Output: fmt.Sprintf("failed to create stderr pipe: %s", err), IsError: true}, nil
	}

	if err := cmd.Start(); err != nil {
		return Result{Output: fmt.Sprintf("failed to run command: %s", err), IsError: true}, nil
	}

	// Scan stdout and stderr concurrently.
	var wg sync.WaitGroup
	var stdoutBuf, stderrBuf strings.Builder

	scanPipe := func(pipe io.Reader, buf *strings.Builder, prefix string) {
		defer wg.Done()
		scanner := bufio.NewScanner(pipe)
		scanner.Buffer(make([]byte, 0, 64*1024), maxOutputSize)
		for scanner.Scan() {
			line := scanner.Text()
			buf.WriteString(line)
			buf.WriteString("\n")
			if onOutput != nil {
				onOutput(prefix + line + "\n")
			}
		}
		if err := scanner.Err(); err != nil {
			errMsg := fmt.Sprintf("\n[stream read error: %s]\n", err)
			buf.WriteString(errMsg)
			if onOutput != nil {
				onOutput(errMsg)
			}
		}
	}

	wg.Add(2)
	go scanPipe(stdoutPipe, &stdoutBuf, "")
	go scanPipe(stderrPipe, &stderrBuf, "")

	wg.Wait()
	err = cmd.Wait()

	if ctx.Err() == context.DeadlineExceeded {
		if cmd.Process != nil {
			if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
				log.Printf("failed to kill process group %d: %v", cmd.Process.Pid, err)
			}
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

	output := buildOutput([]byte(stdoutBuf.String()), []byte(stderrBuf.String()), exitCode)
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
		exitCode := 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		output := truncateOutput(buildOutput(stdout.Bytes(), stderr.Bytes(), exitCode))

		bgMu.Lock()
		bp.Output = output
		bp.Err = err
		bgMu.Unlock()
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
	result := fmt.Sprintf("... (output truncated, showing last %d lines)\n%s", len(kept), strings.Join(kept, "\n"))

	// Enforce byte size cap — line selection alone may not be enough for long lines.
	if len(result) > maxOutputSize {
		result = "... (output truncated)\n" + result[len(result)-maxOutputSize:]
	}
	return result
}

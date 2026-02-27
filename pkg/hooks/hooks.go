package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
)

// Hook defines a shell command to execute when a matching event fires.
type Hook struct {
	Event   Event  `json:"event"`
	Matcher string `json:"matcher"` // regex on tool name; empty matches all
	Command string `json:"command"` // shell command executed via bash -c
}

// compiledHook pairs a Hook with its pre-compiled matcher regex.
type compiledHook struct {
	hook    Hook
	matcher *regexp.Regexp // nil means match everything
}

// HookRunner executes hooks that match incoming events.
type HookRunner struct {
	hooks []compiledHook
}

// NewHookRunner compiles matchers and returns a ready-to-use runner.
// Invalid regex patterns are logged and the hook is skipped.
func NewHookRunner(hooks []Hook) *HookRunner {
	compiled := make([]compiledHook, 0, len(hooks))
	for _, h := range hooks {
		ch := compiledHook{hook: h}
		if h.Matcher != "" {
			re, err := regexp.Compile(h.Matcher)
			if err != nil {
				log.Printf("warning: skipping hook %q: bad matcher regex %q: %v", h.Command, h.Matcher, err)
				continue
			}
			ch.matcher = re
		}
		compiled = append(compiled, ch)
	}
	return &HookRunner{hooks: compiled}
}

// Run finds all hooks matching the event and tool name, then executes them.
//
// For PreToolUse: a non-zero exit from any hook returns an error (blocks the
// tool). For all other events: failures are logged as warnings and nil is
// returned.
func (r *HookRunner) Run(ctx context.Context, event Event, toolName string, toolInput json.RawMessage) error {
	for _, ch := range r.hooks {
		if ch.hook.Event != event {
			continue
		}
		if ch.matcher != nil && !ch.matcher.MatchString(toolName) {
			continue
		}

		err := executeHook(ctx, ch.hook, event, toolName, toolInput)
		if err != nil {
			if event == PreToolUse {
				return fmt.Errorf("PreToolUse hook %q failed: %w", ch.hook.Command, err)
			}
			log.Printf("warning: hook %q for event %s failed: %v", ch.hook.Command, event, err)
		}
	}
	return nil
}

// executeHook runs a single hook command via bash -c, passing context as
// environment variables and toolInput as stdin.
func executeHook(ctx context.Context, h Hook, event Event, toolName string, toolInput json.RawMessage) error {
	cmd := exec.CommandContext(ctx, "bash", "-c", h.Command)

	cmd.Env = append(cmd.Environ(),
		"GLAMDRING_EVENT="+string(event),
		"GLAMDRING_TOOL_NAME="+toolName,
	)

	if len(toolInput) > 0 {
		cmd.Stdin = bytes.NewReader(toolInput)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%w: %s", err, stderr.String())
		}
		return err
	}
	return nil
}

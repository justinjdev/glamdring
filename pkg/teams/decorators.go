package teams

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/justin/glamdring/pkg/config"
	"github.com/justin/glamdring/pkg/tools"
)

// ScopedTool wraps a tool to enforce file path restrictions via glob patterns.
type ScopedTool struct {
	inner         tools.Tool
	allowPatterns []string
	denyPatterns  []string
}

// NewScopedTool creates a ScopedTool that restricts file access to the given
// allow/deny glob patterns. If allowPatterns is empty, all paths are allowed
// (only deny patterns are checked). Deny patterns override allow patterns.
func NewScopedTool(inner tools.Tool, allow, deny []string) *ScopedTool {
	return &ScopedTool{
		inner:         inner,
		allowPatterns: allow,
		denyPatterns:  deny,
	}
}

func (s *ScopedTool) Name() string             { return s.inner.Name() }
func (s *ScopedTool) Description() string      { return s.inner.Description() }
func (s *ScopedTool) Schema() json.RawMessage  { return s.inner.Schema() }

func (s *ScopedTool) Execute(ctx context.Context, input json.RawMessage) (tools.Result, error) {
	if err := s.checkPath(input); err != "" {
		return tools.Result{Output: err, IsError: true}, nil
	}
	return s.inner.Execute(ctx, input)
}

// ExecuteStreaming implements tools.StreamingTool if the inner tool supports it.
func (s *ScopedTool) ExecuteStreaming(ctx context.Context, input json.RawMessage, onOutput func(string)) (tools.Result, error) {
	if err := s.checkPath(input); err != "" {
		return tools.Result{Output: err, IsError: true}, nil
	}
	if st, ok := s.inner.(tools.StreamingTool); ok {
		return st.ExecuteStreaming(ctx, input, onOutput)
	}
	return s.inner.Execute(ctx, input)
}

func (s *ScopedTool) checkPath(input json.RawMessage) string {
	var parsed struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(input, &parsed); err != nil {
		return fmt.Sprintf("invalid input: %s", err)
	}
	if parsed.FilePath == "" {
		// No file_path in input; nothing to restrict.
		return ""
	}

	// Normalize to prevent traversal bypasses (e.g. "src/../../secret").
	parsed.FilePath = filepath.Clean(parsed.FilePath)

	// Check deny patterns first: if any match, block.
	for _, pattern := range s.denyPatterns {
		if config.MatchGlobPattern(pattern, parsed.FilePath) {
			return fmt.Sprintf("file path %q is outside the allowed scope for this agent", parsed.FilePath)
		}
	}

	// If allow patterns are configured, at least one must match.
	if len(s.allowPatterns) > 0 {
		allowed := false
		for _, pattern := range s.allowPatterns {
			if config.MatchGlobPattern(pattern, parsed.FilePath) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Sprintf("file path %q is outside the allowed scope for this agent", parsed.FilePath)
		}
	}

	return ""
}

// ScopedBash wraps a Bash tool to enforce command prefix restrictions.
type ScopedBash struct {
	inner         tools.Tool
	allowCommands []string
}

// NewScopedBash creates a ScopedBash that restricts commands to those matching
// any of the given command prefixes. If allowCommands is empty, all commands
// are allowed. Prefixes should include a trailing space to avoid partial matches
// (e.g. "go " not "go") unless exact-prefix semantics are intended.
func NewScopedBash(inner tools.Tool, allowCommands []string) *ScopedBash {
	return &ScopedBash{
		inner:         inner,
		allowCommands: allowCommands,
	}
}

func (s *ScopedBash) Name() string             { return s.inner.Name() }
func (s *ScopedBash) Description() string      { return s.inner.Description() }
func (s *ScopedBash) Schema() json.RawMessage  { return s.inner.Schema() }

func (s *ScopedBash) Execute(ctx context.Context, input json.RawMessage) (tools.Result, error) {
	if err := s.checkCommand(input); err != "" {
		return tools.Result{Output: err, IsError: true}, nil
	}
	return s.inner.Execute(ctx, input)
}

// ExecuteStreaming implements tools.StreamingTool if the inner tool supports it.
func (s *ScopedBash) ExecuteStreaming(ctx context.Context, input json.RawMessage, onOutput func(string)) (tools.Result, error) {
	if err := s.checkCommand(input); err != "" {
		return tools.Result{Output: err, IsError: true}, nil
	}
	if st, ok := s.inner.(tools.StreamingTool); ok {
		return st.ExecuteStreaming(ctx, input, onOutput)
	}
	return s.inner.Execute(ctx, input)
}

// shellMetachars are characters that can chain or inject additional commands.
var shellMetachars = []string{";", "&&", "||", "|", "`", "$(", "\n"}

func (s *ScopedBash) checkCommand(input json.RawMessage) string {
	if len(s.allowCommands) == 0 {
		return ""
	}
	var parsed struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(input, &parsed); err != nil {
		return fmt.Sprintf("invalid input: %s", err)
	}
	cmd := strings.TrimSpace(parsed.Command)

	// Reject commands containing shell metacharacters that could chain
	// arbitrary commands after an allowed prefix.
	for _, meta := range shellMetachars {
		if strings.Contains(cmd, meta) {
			return fmt.Sprintf("command contains disallowed shell metacharacter %q", meta)
		}
	}

	for _, prefix := range s.allowCommands {
		if strings.HasPrefix(cmd, prefix) {
			return ""
		}
	}
	return fmt.Sprintf("command %q is not in the allowed command list for this agent", cmd)
}

// FileLockDecorator wraps a tool to check file locks before execution.
type FileLockDecorator struct {
	inner tools.Tool
	locks LockManager
	agent string
}

// NewFileLockDecorator creates a decorator that checks and acquires file locks
// before delegating to the inner tool.
func NewFileLockDecorator(inner tools.Tool, locks LockManager, agent string) *FileLockDecorator {
	return &FileLockDecorator{
		inner: inner,
		locks: locks,
		agent: agent,
	}
}

func (d *FileLockDecorator) Name() string             { return d.inner.Name() }
func (d *FileLockDecorator) Description() string      { return d.inner.Description() }
func (d *FileLockDecorator) Schema() json.RawMessage  { return d.inner.Schema() }

func (d *FileLockDecorator) Execute(ctx context.Context, input json.RawMessage) (tools.Result, error) {
	if errMsg := d.checkFileLock(input); errMsg != "" {
		return tools.Result{Output: errMsg, IsError: true}, nil
	}
	return d.inner.Execute(ctx, input)
}

// ExecuteStreaming implements tools.StreamingTool if the inner tool supports it.
func (d *FileLockDecorator) ExecuteStreaming(ctx context.Context, input json.RawMessage, onOutput func(string)) (tools.Result, error) {
	if errMsg := d.checkFileLock(input); errMsg != "" {
		return tools.Result{Output: errMsg, IsError: true}, nil
	}
	if st, ok := d.inner.(tools.StreamingTool); ok {
		return st.ExecuteStreaming(ctx, input, onOutput)
	}
	return d.inner.Execute(ctx, input)
}

func (d *FileLockDecorator) checkFileLock(input json.RawMessage) string {
	var parsed struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(input, &parsed); err != nil {
		return fmt.Sprintf("invalid input: %s", err)
	}
	if parsed.FilePath != "" {
		if err := d.locks.Acquire(parsed.FilePath, d.agent); err != nil {
			return fmt.Sprintf("file %q is locked: %s", parsed.FilePath, err)
		}
	}
	return ""
}

// CheckinGateDecorator wraps a tool to enforce check-in frequency.
type CheckinGateDecorator struct {
	inner     tools.Tool
	checkins  CheckinTracker
	agent     string
	threshold int
}

// NewCheckinGateDecorator creates a decorator that blocks tool execution when
// an agent has exceeded the given threshold of tool calls without checking in.
func NewCheckinGateDecorator(inner tools.Tool, checkins CheckinTracker, agent string, threshold int) *CheckinGateDecorator {
	return &CheckinGateDecorator{
		inner:     inner,
		checkins:  checkins,
		agent:     agent,
		threshold: threshold,
	}
}

func (d *CheckinGateDecorator) Name() string             { return d.inner.Name() }
func (d *CheckinGateDecorator) Description() string      { return d.inner.Description() }
func (d *CheckinGateDecorator) Schema() json.RawMessage  { return d.inner.Schema() }

func (d *CheckinGateDecorator) Execute(ctx context.Context, input json.RawMessage) (tools.Result, error) {
	if d.checkins.Count(d.agent) >= d.threshold {
		return tools.Result{
			Output:  fmt.Sprintf("agent has exceeded %d tool calls without checking in; use TaskUpdate or SendMessage to report progress", d.threshold),
			IsError: true,
		}, nil
	}
	d.checkins.Increment(d.agent)
	return d.inner.Execute(ctx, input)
}

// ExecuteStreaming implements tools.StreamingTool if the inner tool supports it.
func (d *CheckinGateDecorator) ExecuteStreaming(ctx context.Context, input json.RawMessage, onOutput func(string)) (tools.Result, error) {
	if d.checkins.Count(d.agent) >= d.threshold {
		return tools.Result{
			Output:  fmt.Sprintf("agent has exceeded %d tool calls without checking in; use TaskUpdate or SendMessage to report progress", d.threshold),
			IsError: true,
		}, nil
	}
	d.checkins.Increment(d.agent)
	if st, ok := d.inner.(tools.StreamingTool); ok {
		return st.ExecuteStreaming(ctx, input, onOutput)
	}
	return d.inner.Execute(ctx, input)
}

// ComposeDecorators applies decorators in order so that decorators[0] is the
// outermost wrapper: decorators[0](decorators[1](...(base))).
func ComposeDecorators(base tools.Tool, decorators ...func(tools.Tool) tools.Tool) tools.Tool {
	t := base
	for i := len(decorators) - 1; i >= 0; i-- {
		t = decorators[i](t)
	}
	return t
}

package teams

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/justin/glamdring/pkg/tools"
)

// --- Test stubs and mocks ---

type stubTool struct {
	name   string
	execFn func(ctx context.Context, input json.RawMessage) (tools.Result, error)
}

func (s *stubTool) Name() string            { return s.name }
func (s *stubTool) Description() string     { return "stub tool" }
func (s *stubTool) Schema() json.RawMessage { return json.RawMessage(`{}`) }

func (s *stubTool) Execute(ctx context.Context, input json.RawMessage) (tools.Result, error) {
	if s.execFn != nil {
		return s.execFn(ctx, input)
	}
	return tools.Result{Output: "ok"}, nil
}

// stubStreamingTool implements both Tool and StreamingTool for testing ScopedBash.
type stubStreamingTool struct {
	stubTool
	streamFn func(ctx context.Context, input json.RawMessage, onOutput func(string)) (tools.Result, error)
}

func (s *stubStreamingTool) ExecuteStreaming(ctx context.Context, input json.RawMessage, onOutput func(string)) (tools.Result, error) {
	if s.streamFn != nil {
		return s.streamFn(ctx, input, onOutput)
	}
	onOutput("streamed")
	return tools.Result{Output: "streamed"}, nil
}

type mockLockManager struct {
	locks map[string]string // path -> owner
}

func newMockLockManager() *mockLockManager {
	return &mockLockManager{locks: make(map[string]string)}
}

func (m *mockLockManager) Acquire(path string, owner string) error {
	m.locks[path] = owner
	return nil
}

func (m *mockLockManager) Release(path string, owner string) error {
	if m.locks[path] == owner {
		delete(m.locks, path)
	}
	return nil
}

func (m *mockLockManager) Check(path string) (string, bool) {
	owner, ok := m.locks[path]
	return owner, ok
}

func (m *mockLockManager) ReleaseAll(owner string) {
	for path, o := range m.locks {
		if o == owner {
			delete(m.locks, path)
		}
	}
}

type mockCheckinTracker struct {
	counts map[string]int
}

func newMockCheckinTracker() *mockCheckinTracker {
	return &mockCheckinTracker{counts: make(map[string]int)}
}

func (m *mockCheckinTracker) Increment(agentName string) int {
	m.counts[agentName]++
	return m.counts[agentName]
}

func (m *mockCheckinTracker) Reset(agentName string) {
	m.counts[agentName] = 0
}

func (m *mockCheckinTracker) Count(agentName string) int {
	return m.counts[agentName]
}

func (m *mockCheckinTracker) Remove(agentName string) {
	delete(m.counts, agentName)
}

// --- ScopedTool tests ---

func TestScopedTool_AllowedPath(t *testing.T) {
	inner := &stubTool{name: "Write"}
	scoped := NewScopedTool(inner, []string{"/project/src/*"}, nil)

	input := json.RawMessage(`{"file_path": "/project/src/main.go"}`)
	result, err := scoped.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected success, got error: %s", result.Output)
	}
}

func TestScopedTool_DeniedPath(t *testing.T) {
	inner := &stubTool{name: "Write"}
	scoped := NewScopedTool(inner, []string{"/project/src/*"}, nil)

	input := json.RawMessage(`{"file_path": "/etc/passwd"}`)
	result, err := scoped.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for path outside allow patterns")
	}
	if !strings.Contains(result.Output, "outside the allowed scope") {
		t.Errorf("unexpected error message: %s", result.Output)
	}
}

func TestScopedTool_NoAllowPatterns(t *testing.T) {
	inner := &stubTool{name: "Write"}
	scoped := NewScopedTool(inner, nil, []string{"/secret/*"})

	// Path not in deny list should pass.
	input := json.RawMessage(`{"file_path": "/project/src/main.go"}`)
	result, err := scoped.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected success, got error: %s", result.Output)
	}

	// Path in deny list should be blocked.
	input = json.RawMessage(`{"file_path": "/secret/keys"}`)
	result, err = scoped.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for denied path")
	}
}

func TestScopedTool_DenyOverridesAllow(t *testing.T) {
	inner := &stubTool{name: "Write"}
	scoped := NewScopedTool(inner, []string{"/project/*"}, []string{"/project/secret"})

	input := json.RawMessage(`{"file_path": "/project/secret"}`)
	result, err := scoped.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected deny to override allow")
	}
}

func TestScopedTool_NoFilePath(t *testing.T) {
	inner := &stubTool{name: "Write"}
	scoped := NewScopedTool(inner, []string{"/project/*"}, nil)

	// Input without file_path should pass through.
	input := json.RawMessage(`{"content": "hello"}`)
	result, err := scoped.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected success for input without file_path, got error: %s", result.Output)
	}
}

func TestScopedTool_DelegatesNameDescriptionSchema(t *testing.T) {
	inner := &stubTool{name: "Edit"}
	scoped := NewScopedTool(inner, nil, nil)

	if scoped.Name() != "Edit" {
		t.Errorf("expected name 'Edit', got %q", scoped.Name())
	}
	if scoped.Description() != "stub tool" {
		t.Errorf("expected description 'stub tool', got %q", scoped.Description())
	}
}

// --- ScopedBash tests ---

func TestScopedBash_AllowedCommand(t *testing.T) {
	inner := &stubTool{name: "Bash"}
	scoped := NewScopedBash(inner, []string{"go ", "git "})

	input := json.RawMessage(`{"command": "go test ./..."}`)
	result, err := scoped.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected success, got error: %s", result.Output)
	}
}

func TestScopedBash_DeniedCommand(t *testing.T) {
	inner := &stubTool{name: "Bash"}
	scoped := NewScopedBash(inner, []string{"go ", "git "})

	input := json.RawMessage(`{"command": "rm -rf /"}`)
	result, err := scoped.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for disallowed command")
	}
	if !strings.Contains(result.Output, "not in the allowed command list") {
		t.Errorf("unexpected error message: %s", result.Output)
	}
}

func TestScopedBash_EmptyAllowListPassesAll(t *testing.T) {
	inner := &stubTool{name: "Bash"}
	scoped := NewScopedBash(inner, nil)

	input := json.RawMessage(`{"command": "any-command --flag"}`)
	result, err := scoped.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected success with empty allow list, got error: %s", result.Output)
	}
}

func TestScopedBash_StreamingDelegation(t *testing.T) {
	streamed := false
	inner := &stubStreamingTool{
		stubTool: stubTool{name: "Bash"},
		streamFn: func(_ context.Context, _ json.RawMessage, onOutput func(string)) (tools.Result, error) {
			streamed = true
			onOutput("output")
			return tools.Result{Output: "done"}, nil
		},
	}
	scoped := NewScopedBash(inner, []string{"go "})

	input := json.RawMessage(`{"command": "go build"}`)
	result, err := scoped.ExecuteStreaming(context.Background(), input, func(s string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected success, got error: %s", result.Output)
	}
	if !streamed {
		t.Error("expected streaming function to be called")
	}
}

func TestScopedBash_StreamingDenied(t *testing.T) {
	inner := &stubStreamingTool{
		stubTool: stubTool{name: "Bash"},
	}
	scoped := NewScopedBash(inner, []string{"go "})

	input := json.RawMessage(`{"command": "rm -rf /"}`)
	result, err := scoped.ExecuteStreaming(context.Background(), input, func(s string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for disallowed command in streaming mode")
	}
}

// --- FileLockDecorator tests ---

func TestFileLockDecorator_UnlockedAutoAcquires(t *testing.T) {
	locks := newMockLockManager()
	inner := &stubTool{name: "Write"}
	dec := NewFileLockDecorator(inner, locks, "agent-a")

	input := json.RawMessage(`{"file_path": "/project/main.go"}`)
	result, err := dec.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected success, got error: %s", result.Output)
	}

	// Lock should now be held by agent-a.
	owner, locked := locks.Check("/project/main.go")
	if !locked || owner != "agent-a" {
		t.Errorf("expected lock held by agent-a, got owner=%q locked=%v", owner, locked)
	}
}

func TestFileLockDecorator_SameAgentExecutes(t *testing.T) {
	locks := newMockLockManager()
	locks.Acquire("/project/main.go", "agent-a")
	inner := &stubTool{name: "Write"}
	dec := NewFileLockDecorator(inner, locks, "agent-a")

	input := json.RawMessage(`{"file_path": "/project/main.go"}`)
	result, err := dec.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected success for same agent, got error: %s", result.Output)
	}
}

func TestFileLockDecorator_DifferentAgentBlocked(t *testing.T) {
	locks := newMockLockManager()
	locks.Acquire("/project/main.go", "agent-b")
	inner := &stubTool{name: "Write"}
	dec := NewFileLockDecorator(inner, locks, "agent-a")

	input := json.RawMessage(`{"file_path": "/project/main.go"}`)
	result, err := dec.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when file is locked by different agent")
	}
	if !strings.Contains(result.Output, "locked by agent") {
		t.Errorf("unexpected error message: %s", result.Output)
	}
}

// --- CheckinGateDecorator tests ---

func TestCheckinGateDecorator_UnderThreshold(t *testing.T) {
	tracker := newMockCheckinTracker()
	inner := &stubTool{name: "Write"}
	dec := NewCheckinGateDecorator(inner, tracker, "agent-a", 5)

	input := json.RawMessage(`{}`)
	for i := 0; i < 5; i++ {
		result, err := dec.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
		if result.IsError {
			t.Errorf("call %d: expected success, got error: %s", i, result.Output)
		}
	}
}

func TestCheckinGateDecorator_ExceedsThreshold(t *testing.T) {
	tracker := newMockCheckinTracker()
	inner := &stubTool{name: "Write"}
	dec := NewCheckinGateDecorator(inner, tracker, "agent-a", 3)

	input := json.RawMessage(`{}`)
	// First 3 calls should succeed.
	for i := 0; i < 3; i++ {
		result, _ := dec.Execute(context.Background(), input)
		if result.IsError {
			t.Errorf("call %d: expected success, got error: %s", i, result.Output)
		}
	}

	// 4th call (count=4 > threshold=3) should be blocked.
	result, err := dec.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when exceeding threshold")
	}
	if !strings.Contains(result.Output, "exceeded 3 tool calls") {
		t.Errorf("unexpected error message: %s", result.Output)
	}
}

// --- ComposeDecorators tests ---

func TestComposeDecorators_Order(t *testing.T) {
	var order []string
	base := &stubTool{
		name: "Write",
		execFn: func(_ context.Context, _ json.RawMessage) (tools.Result, error) {
			order = append(order, "base")
			return tools.Result{Output: "ok"}, nil
		},
	}

	outer := func(inner tools.Tool) tools.Tool {
		return &stubTool{
			name: inner.Name(),
			execFn: func(ctx context.Context, input json.RawMessage) (tools.Result, error) {
				order = append(order, "outer")
				return inner.Execute(ctx, input)
			},
		}
	}

	middle := func(inner tools.Tool) tools.Tool {
		return &stubTool{
			name: inner.Name(),
			execFn: func(ctx context.Context, input json.RawMessage) (tools.Result, error) {
				order = append(order, "middle")
				return inner.Execute(ctx, input)
			},
		}
	}

	composed := ComposeDecorators(base, outer, middle)
	composed.Execute(context.Background(), json.RawMessage(`{}`))

	expected := []string{"outer", "middle", "base"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(order), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("position %d: expected %q, got %q", i, v, order[i])
		}
	}
}

func TestComposeDecorators_NoDecorators(t *testing.T) {
	base := &stubTool{name: "Read"}
	result := ComposeDecorators(base)
	if result.Name() != "Read" {
		t.Errorf("expected name 'Read', got %q", result.Name())
	}
}

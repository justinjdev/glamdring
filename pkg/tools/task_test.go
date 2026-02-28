package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/justin/glamdring/pkg/agents"
)

func TestNewTaskTool(t *testing.T) {
	runner := func(_ context.Context, _ SubagentOptions) <-chan SubagentResult {
		ch := make(chan SubagentResult)
		close(ch)
		return ch
	}
	defs := agents.NewRegistry(nil)
	allTools := []Tool{&stubTool{name: "Read"}}

	tt := NewTaskTool(runner, defs, allTools)

	if tt == nil {
		t.Fatal("NewTaskTool returned nil")
	}
	if tt.runner == nil {
		t.Error("runner is nil")
	}
	if tt.agentDefs == nil {
		t.Error("agentDefs is nil")
	}
	if len(tt.allTools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tt.allTools))
	}
}

func TestTaskTool_Description(t *testing.T) {
	tt := &TaskTool{}
	desc := tt.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
}

func TestTaskTool_Schema(t *testing.T) {
	t.Run("basic schema", func(t *testing.T) {
		tt := &TaskTool{}
		raw := tt.Schema()

		var schema map[string]any
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("Schema() returned invalid JSON: %v", err)
		}

		if schema["type"] != "object" {
			t.Errorf("expected type=object, got %v", schema["type"])
		}

		required, ok := schema["required"].([]any)
		if !ok {
			t.Fatal("required field missing or not an array")
		}
		found := false
		for _, r := range required {
			if r == "prompt" {
				found = true
				break
			}
		}
		if !found {
			t.Error("prompt not in required fields")
		}

		props, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatal("properties missing or not a map")
		}
		for _, key := range []string{"prompt", "subagent_type", "allowed_tools"} {
			if _, ok := props[key]; !ok {
				t.Errorf("missing property %q", key)
			}
		}
	})

	t.Run("with agent definitions", func(t *testing.T) {
		defs := agents.NewRegistry([]agents.AgentDefinition{
			{Name: "researcher", Prompt: "You research."},
			{Name: "coder", Prompt: "You code."},
		})
		tt := &TaskTool{agentDefs: defs}
		raw := tt.Schema()

		var schema map[string]any
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("Schema() returned invalid JSON: %v", err)
		}

		props := schema["properties"].(map[string]any)
		st := props["subagent_type"].(map[string]any)
		desc := st["description"].(string)

		if !strings.Contains(desc, "coder") {
			t.Errorf("description should contain agent name 'coder': %s", desc)
		}
		if !strings.Contains(desc, "researcher") {
			t.Errorf("description should contain agent name 'researcher': %s", desc)
		}
	})
}

func TestFilterTools(t *testing.T) {
	all := []Tool{
		&stubTool{name: "Read"},
		&stubTool{name: "Write"},
		&stubTool{name: "Bash"},
		&stubTool{name: "Glob"},
	}

	tests := []struct {
		name     string
		allowed  []string
		expected []string
	}{
		{
			name:     "include subset",
			allowed:  []string{"Read", "Write"},
			expected: []string{"Read", "Write"},
		},
		{
			name:     "include all",
			allowed:  []string{"Read", "Write", "Bash", "Glob"},
			expected: []string{"Read", "Write", "Bash", "Glob"},
		},
		{
			name:     "include none matching",
			allowed:  []string{"NonExistent"},
			expected: []string{},
		},
		{
			name:     "empty allowed list",
			allowed:  []string{},
			expected: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := filterTools(all, tc.allowed)
			if len(result) != len(tc.expected) {
				t.Fatalf("expected %d tools, got %d", len(tc.expected), len(result))
			}
			for i, tool := range result {
				if tool.Name() != tc.expected[i] {
					t.Errorf("position %d: expected %q, got %q", i, tc.expected[i], tool.Name())
				}
			}
		})
	}
}

func TestExcludeTool(t *testing.T) {
	all := []Tool{
		&stubTool{name: "Read"},
		&stubTool{name: "Task"},
		&stubTool{name: "Write"},
	}

	t.Run("removes named tool", func(t *testing.T) {
		result := excludeTool(all, "Task")
		if len(result) != 2 {
			t.Fatalf("expected 2 tools, got %d", len(result))
		}
		for _, tool := range result {
			if tool.Name() == "Task" {
				t.Error("Task tool should have been excluded")
			}
		}
	})

	t.Run("not found leaves all intact", func(t *testing.T) {
		result := excludeTool(all, "NonExistent")
		if len(result) != 3 {
			t.Fatalf("expected 3 tools, got %d", len(result))
		}
	})
}

func TestTaskTool_Execute_InvalidInput(t *testing.T) {
	tt := &TaskTool{}
	result, err := tt.Execute(context.Background(), json.RawMessage(`{bad json`))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for invalid JSON")
	}
	if !strings.Contains(result.Output, "invalid input") {
		t.Errorf("expected 'invalid input' in output, got %q", result.Output)
	}
}

func TestTaskTool_Execute_MissingPrompt(t *testing.T) {
	tt := &TaskTool{}
	result, err := tt.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for missing prompt")
	}
	if !strings.Contains(result.Output, "prompt is required") {
		t.Errorf("expected 'prompt is required' in output, got %q", result.Output)
	}
}

func TestTaskTool_Execute_EmptyPrompt(t *testing.T) {
	tt := &TaskTool{}
	result, err := tt.Execute(context.Background(), json.RawMessage(`{"prompt":""}`))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for empty prompt")
	}
}

func TestTaskTool_Execute_Success(t *testing.T) {
	runner := func(_ context.Context, opts SubagentOptions) <-chan SubagentResult {
		ch := make(chan SubagentResult, 3)
		ch <- SubagentResult{Text: "Hello "}
		ch <- SubagentResult{Text: "World"}
		ch <- SubagentResult{Done: true}
		close(ch)
		return ch
	}

	allTools := []Tool{
		&stubTool{name: "Read"},
		&stubTool{name: "Write"},
	}

	tt := NewTaskTool(runner, agents.NewRegistry(nil), allTools)

	input := `{"prompt":"do something"}`
	result, err := tt.Execute(context.Background(), json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("expected IsError=false for successful run")
	}
	if result.Output != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", result.Output)
	}
}

func TestTaskTool_Execute_ErrorResult(t *testing.T) {
	runner := func(_ context.Context, _ SubagentOptions) <-chan SubagentResult {
		ch := make(chan SubagentResult, 2)
		ch <- SubagentResult{Text: "partial output", IsError: true}
		ch <- SubagentResult{Done: true}
		close(ch)
		return ch
	}

	tt := NewTaskTool(runner, agents.NewRegistry(nil), nil)

	result, err := tt.Execute(context.Background(), json.RawMessage(`{"prompt":"fail"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true when subagent has errors")
	}
	if result.Output != "partial output" {
		t.Errorf("expected 'partial output', got %q", result.Output)
	}
}

func TestTaskTool_Execute_NoOutput(t *testing.T) {
	runner := func(_ context.Context, _ SubagentOptions) <-chan SubagentResult {
		ch := make(chan SubagentResult, 1)
		ch <- SubagentResult{Done: true}
		close(ch)
		return ch
	}

	tt := NewTaskTool(runner, agents.NewRegistry(nil), nil)

	result, err := tt.Execute(context.Background(), json.RawMessage(`{"prompt":"silent"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "subagent completed with no output" {
		t.Errorf("unexpected output: %q", result.Output)
	}
}

func TestTaskTool_Execute_ErrorNoOutput(t *testing.T) {
	runner := func(_ context.Context, _ SubagentOptions) <-chan SubagentResult {
		ch := make(chan SubagentResult, 2)
		ch <- SubagentResult{IsError: true}
		ch <- SubagentResult{Done: true}
		close(ch)
		return ch
	}

	tt := NewTaskTool(runner, agents.NewRegistry(nil), nil)

	result, err := tt.Execute(context.Background(), json.RawMessage(`{"prompt":"fail silent"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true")
	}
	if result.Output != "subagent completed with errors but produced no output" {
		t.Errorf("unexpected output: %q", result.Output)
	}
}

func TestTaskTool_Execute_WithSubagentType(t *testing.T) {
	var capturedOpts SubagentOptions

	runner := func(_ context.Context, opts SubagentOptions) <-chan SubagentResult {
		capturedOpts = opts
		ch := make(chan SubagentResult, 2)
		ch <- SubagentResult{Text: "done"}
		ch <- SubagentResult{Done: true}
		close(ch)
		return ch
	}

	defs := agents.NewRegistry([]agents.AgentDefinition{
		{Name: "researcher", Prompt: "You are a researcher.", Tools: []string{"Read", "Glob"}},
	})

	allTools := []Tool{
		&stubTool{name: "Read"},
		&stubTool{name: "Write"},
		&stubTool{name: "Glob"},
		&stubTool{name: "Task"},
	}

	tt := NewTaskTool(runner, defs, allTools)

	input := `{"prompt":"research this","subagent_type":"researcher"}`
	result, err := tt.Execute(context.Background(), json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Output)
	}

	if capturedOpts.SystemPrompt != "You are a researcher." {
		t.Errorf("expected system prompt from agent def, got %q", capturedOpts.SystemPrompt)
	}

	// Should have filtered to only Read and Glob (Task is always excluded).
	toolNames := make([]string, len(capturedOpts.Tools))
	for i, tool := range capturedOpts.Tools {
		toolNames[i] = tool.Name()
	}
	if len(capturedOpts.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d: %v", len(capturedOpts.Tools), toolNames)
	}
}

func TestTaskTool_Execute_UnknownSubagentType(t *testing.T) {
	defs := agents.NewRegistry([]agents.AgentDefinition{
		{Name: "coder", Prompt: "You code."},
	})

	tt := NewTaskTool(nil, defs, nil)

	input := `{"prompt":"go","subagent_type":"unknown"}`
	result, err := tt.Execute(context.Background(), json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for unknown subagent_type")
	}
	if !strings.Contains(result.Output, "unknown subagent_type") {
		t.Errorf("expected error about unknown subagent_type, got %q", result.Output)
	}
}

func TestTaskTool_Execute_AllowedToolsOverride(t *testing.T) {
	var capturedOpts SubagentOptions

	runner := func(_ context.Context, opts SubagentOptions) <-chan SubagentResult {
		capturedOpts = opts
		ch := make(chan SubagentResult, 2)
		ch <- SubagentResult{Text: "ok"}
		ch <- SubagentResult{Done: true}
		close(ch)
		return ch
	}

	allTools := []Tool{
		&stubTool{name: "Read"},
		&stubTool{name: "Write"},
		&stubTool{name: "Bash"},
	}

	tt := NewTaskTool(runner, agents.NewRegistry(nil), allTools)

	input := `{"prompt":"just read","allowed_tools":["Read"]}`
	_, err := tt.Execute(context.Background(), json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedOpts.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(capturedOpts.Tools))
	}
	if capturedOpts.Tools[0].Name() != "Read" {
		t.Errorf("expected Read tool, got %s", capturedOpts.Tools[0].Name())
	}
}

func TestTaskTool_Execute_ExcludesTaskTool(t *testing.T) {
	var capturedOpts SubagentOptions

	runner := func(_ context.Context, opts SubagentOptions) <-chan SubagentResult {
		capturedOpts = opts
		ch := make(chan SubagentResult, 2)
		ch <- SubagentResult{Text: "ok"}
		ch <- SubagentResult{Done: true}
		close(ch)
		return ch
	}

	allTools := []Tool{
		&stubTool{name: "Read"},
		&stubTool{name: "Task"},
	}

	tt := NewTaskTool(runner, agents.NewRegistry(nil), allTools)

	input := `{"prompt":"go"}`
	_, err := tt.Execute(context.Background(), json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, tool := range capturedOpts.Tools {
		if tool.Name() == "Task" {
			t.Error("Task tool should be excluded from subagent tools")
		}
	}
}

func TestTaskTool_Execute_TeamNameWithoutName(t *testing.T) {
	tt := &TaskTool{
		TeamSetupFunc: func(_ context.Context, _ TeamSetupParams) (*TeamSetupResult, error) {
			return nil, nil
		},
	}
	input := `{"prompt":"go","team_name":"my-team"}`
	result, err := tt.Execute(context.Background(), json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when team_name set but name missing")
	}
	if !strings.Contains(result.Output, "name is required") {
		t.Errorf("expected 'name is required' in output, got %q", result.Output)
	}
}

func TestTaskTool_Execute_TeamNameWithoutTeamsEnabled(t *testing.T) {
	tt := &TaskTool{} // no TeamSetupFunc
	input := `{"prompt":"go","team_name":"my-team","name":"agent-1"}`
	result, err := tt.Execute(context.Background(), json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when teams not enabled")
	}
	if !strings.Contains(result.Output, "teams are not enabled") {
		t.Errorf("expected 'teams are not enabled' in output, got %q", result.Output)
	}
}

func TestTaskTool_Execute_TeamNameCallsSetupFunc(t *testing.T) {
	var capturedParams TeamSetupParams

	setupFunc := func(_ context.Context, params TeamSetupParams) (*TeamSetupResult, error) {
		capturedParams = params
		return &TeamSetupResult{
			Tools:        []Tool{&stubTool{name: "Read"}},
			SystemPrompt: "team prompt",
			Model:        "claude-sonnet-4-6",
		}, nil
	}

	var capturedOpts SubagentOptions
	runner := func(_ context.Context, opts SubagentOptions) <-chan SubagentResult {
		capturedOpts = opts
		ch := make(chan SubagentResult, 2)
		ch <- SubagentResult{Text: "team output"}
		ch <- SubagentResult{Done: true}
		close(ch)
		return ch
	}

	tt := &TaskTool{
		runner:        runner,
		allTools:      []Tool{&stubTool{name: "Read"}},
		TeamSetupFunc: setupFunc,
	}

	input := `{"prompt":"do team work","team_name":"my-team","name":"worker-1","workflow":"rpiv","start_phase":"implement"}`
	result, err := tt.Execute(context.Background(), json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Output)
	}
	if result.Output != "team output" {
		t.Errorf("expected 'team output', got %q", result.Output)
	}

	// Verify setup params.
	if capturedParams.TeamName != "my-team" {
		t.Errorf("expected team name 'my-team', got %q", capturedParams.TeamName)
	}
	if capturedParams.AgentName != "worker-1" {
		t.Errorf("expected agent name 'worker-1', got %q", capturedParams.AgentName)
	}
	if capturedParams.Workflow != "rpiv" {
		t.Errorf("expected workflow 'rpiv', got %q", capturedParams.Workflow)
	}
	if capturedParams.StartPhase != "implement" {
		t.Errorf("expected start phase 'implement', got %q", capturedParams.StartPhase)
	}

	// Verify runner received setup results.
	if capturedOpts.SystemPrompt != "team prompt" {
		t.Errorf("expected 'team prompt', got %q", capturedOpts.SystemPrompt)
	}
	if capturedOpts.Model != "claude-sonnet-4-6" {
		t.Errorf("expected model 'claude-sonnet-4-6', got %q", capturedOpts.Model)
	}
}

func TestTaskTool_Schema_IncludesTeamFields(t *testing.T) {
	// Without TeamSetupFunc, no team fields.
	tt := &TaskTool{}
	raw := tt.Schema()
	var schema map[string]any
	json.Unmarshal(raw, &schema)
	props := schema["properties"].(map[string]any)
	if _, ok := props["team_name"]; ok {
		t.Error("team_name should not be in schema without TeamSetupFunc")
	}

	// With TeamSetupFunc, team fields present.
	tt.TeamSetupFunc = func(_ context.Context, _ TeamSetupParams) (*TeamSetupResult, error) {
		return nil, nil
	}
	raw = tt.Schema()
	json.Unmarshal(raw, &schema)
	props = schema["properties"].(map[string]any)
	for _, field := range []string{"team_name", "name", "workflow", "start_phase"} {
		if _, ok := props[field]; !ok {
			t.Errorf("expected %q in schema with TeamSetupFunc", field)
		}
	}
}

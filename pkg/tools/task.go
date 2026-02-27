package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/justin/glamdring/pkg/agents"
)

// SubagentRunner is a callback that starts a subagent and returns a channel of
// results. It is provided at initialization time to break the circular
// dependency between pkg/tools and pkg/agent — the caller (e.g. main.go)
// wraps agent.Run into this signature.
type SubagentRunner func(ctx context.Context, opts SubagentOptions) <-chan SubagentResult

// SubagentOptions describes the configuration for a subagent run.
type SubagentOptions struct {
	Prompt       string
	SystemPrompt string
	Tools        []Tool
	MaxTurns     int
}

// SubagentResult is a single unit of output from a running subagent.
type SubagentResult struct {
	Text    string
	IsError bool
	Done    bool
}

// TaskTool spawns a subagent to execute a task and returns the collected
// text output. It implements the Tool interface.
type TaskTool struct {
	runner    SubagentRunner
	agentDefs *agents.Registry
	allTools  []Tool
}

// NewTaskTool creates a TaskTool. runner is the callback that starts a
// subagent (typically wrapping agent.Run). agentDefs provides custom agent
// definitions for the subagent_type parameter. allTools is the parent
// agent's full tool set, used as the default and as the pool for filtering
// via allowed_tools.
func NewTaskTool(runner SubagentRunner, agentDefs *agents.Registry, allTools []Tool) *TaskTool {
	return &TaskTool{
		runner:    runner,
		agentDefs: agentDefs,
		allTools:  allTools,
	}
}

type taskInput struct {
	Prompt       string   `json:"prompt"`
	SubagentType string   `json:"subagent_type"`
	AllowedTools []string `json:"allowed_tools"`
}

func (TaskTool) Name() string { return "Task" }

func (TaskTool) Description() string {
	return "Spawn a subagent to perform a task. The subagent gets its own conversation context and runs independently. Use this for parallel research, code analysis, or any task that can be delegated."
}

func (t TaskTool) Schema() json.RawMessage {
	schema := map[string]any{
		"type":     "object",
		"required": []string{"prompt"},
		"properties": map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "The task description / prompt for the subagent",
			},
			"subagent_type": map[string]any{
				"type":        "string",
				"description": "Name of a custom agent definition to use (applies its system prompt and tool restrictions)",
			},
			"allowed_tools": map[string]any{
				"type":        "array",
				"description": "Whitelist of tool names the subagent may use. If omitted, the subagent inherits the parent's tools.",
				"items": map[string]any{
					"type": "string",
				},
			},
		},
	}

	// Build the schema with agent names in the description if available.
	if t.agentDefs != nil {
		names := t.agentDefs.Names()
		if len(names) > 0 {
			desc := fmt.Sprintf(
				"Name of a custom agent definition to use. Available agents: %s",
				strings.Join(names, ", "),
			)
			props := schema["properties"].(map[string]any)
			st := props["subagent_type"].(map[string]any)
			st["description"] = desc
		}
	}

	b, _ := json.Marshal(schema)
	return json.RawMessage(b)
}

func (t TaskTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in taskInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Output: fmt.Sprintf("invalid input: %s", err), IsError: true}, nil
	}
	if in.Prompt == "" {
		return Result{Output: "prompt is required", IsError: true}, nil
	}

	opts := SubagentOptions{
		Prompt: in.Prompt,
		Tools:  t.allTools,
	}

	// Apply custom agent definition if specified.
	if in.SubagentType != "" && t.agentDefs != nil {
		def, ok := t.agentDefs.Get(in.SubagentType)
		if !ok {
			available := t.agentDefs.Names()
			return Result{
				Output: fmt.Sprintf(
					"unknown subagent_type %q; available agents: %s",
					in.SubagentType,
					strings.Join(available, ", "),
				),
				IsError: true,
			}, nil
		}

		opts.SystemPrompt = def.Prompt

		// If the agent definition specifies tool restrictions, apply them.
		if len(def.Tools) > 0 {
			opts.Tools = filterTools(t.allTools, def.Tools)
		}
	}

	// If allowed_tools is explicitly set, filter (overrides agent definition tools).
	if len(in.AllowedTools) > 0 {
		opts.Tools = filterTools(t.allTools, in.AllowedTools)
	}

	// Remove the Task tool itself from the subagent's tool set to prevent
	// recursive subagent spawning.
	opts.Tools = excludeTool(opts.Tools, "Task")

	// Run the subagent and collect output.
	ch := t.runner(ctx, opts)

	var buf strings.Builder
	var hadError bool

	for result := range ch {
		if result.Done {
			break
		}
		if result.IsError {
			hadError = true
		}
		if result.Text != "" {
			buf.WriteString(result.Text)
		}
	}

	output := buf.String()
	if output == "" && hadError {
		output = "subagent completed with errors but produced no output"
	} else if output == "" {
		output = "subagent completed with no output"
	}

	return Result{Output: output, IsError: hadError}, nil
}

// filterTools returns only the tools whose names appear in the allowed list.
func filterTools(all []Tool, allowed []string) []Tool {
	set := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		set[name] = true
	}

	var filtered []Tool
	for _, t := range all {
		if set[t.Name()] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// excludeTool returns a copy of tools with the named tool removed.
func excludeTool(tools []Tool, name string) []Tool {
	out := make([]Tool, 0, len(tools))
	for _, t := range tools {
		if t.Name() != name {
			out = append(out, t)
		}
	}
	return out
}

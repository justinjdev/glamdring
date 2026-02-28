package teams

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/justin/glamdring/pkg/tools"
)

// DefaultReadTools are tool names always available for read-only access.
var DefaultReadTools = []string{"Read", "Glob", "Grep"}

// DefaultTeamTools are tool names always available for team coordination.
var DefaultTeamTools = []string{"TaskCreate", "TaskList", "TaskGet", "TaskUpdate", "SendMessage", "AdvancePhase"}

// PhaseRegistry wraps a tools.Registry and filters available tools based on
// the current workflow phase. It implements tools.ToolProvider.
type PhaseRegistry struct {
	base      *tools.Registry
	phases    PhaseTracker
	agentName string
	teamTools []string
	readTools []string
}

// NewPhaseRegistry creates a PhaseRegistry that filters tools from the base
// registry according to the current phase. teamTools and readTools are always
// available regardless of phase. Pass nil to use defaults.
func NewPhaseRegistry(base *tools.Registry, phases PhaseTracker, agentName string, teamTools, readTools []string) *PhaseRegistry {
	if teamTools == nil {
		teamTools = DefaultTeamTools
	}
	if readTools == nil {
		readTools = DefaultReadTools
	}
	return &PhaseRegistry{
		base:      base,
		phases:    phases,
		agentName: agentName,
		teamTools: teamTools,
		readTools: readTools,
	}
}

// Schemas returns tool schemas only for tools allowed in the current phase,
// plus team tools and read tools. If no phases are configured, all schemas
// from the base registry are returned.
func (pr *PhaseRegistry) Schemas() []json.RawMessage {
	allowed := pr.allowedNames()
	if allowed == nil {
		return pr.base.Schemas()
	}

	var out []json.RawMessage
	for _, t := range pr.base.All() {
		if allowed[t.Name()] {
			schema := map[string]any{
				"name":         t.Name(),
				"description":  t.Description(),
				"input_schema": json.RawMessage(t.Schema()),
			}
			b, _ := json.Marshal(schema)
			out = append(out, b)
		}
	}
	return out
}

// Get returns a tool by name if it is allowed in the current phase. Returns
// nil if the tool is not available.
func (pr *PhaseRegistry) Get(name string) tools.Tool {
	allowed := pr.allowedNames()
	if allowed == nil {
		return pr.base.Get(name)
	}
	if !allowed[name] {
		return nil
	}
	return pr.base.Get(name)
}

// Execute runs a tool by name if it is allowed in the current phase.
func (pr *PhaseRegistry) Execute(ctx context.Context, name string, input json.RawMessage) (tools.Result, error) {
	allowed := pr.allowedNames()
	if allowed != nil && !allowed[name] {
		return tools.Result{
			Output:  fmt.Sprintf("tool %q is not available in the current phase", name),
			IsError: true,
		}, nil
	}
	return pr.base.Execute(ctx, name, input)
}

// ExecuteStreaming runs a tool by name with streaming if it is allowed in the
// current phase.
func (pr *PhaseRegistry) ExecuteStreaming(ctx context.Context, name string, input json.RawMessage, onOutput func(string)) (tools.Result, error) {
	allowed := pr.allowedNames()
	if allowed != nil && !allowed[name] {
		return tools.Result{
			Output:  fmt.Sprintf("tool %q is not available in the current phase", name),
			IsError: true,
		}, nil
	}
	return pr.base.ExecuteStreaming(ctx, name, input, onOutput)
}

// CurrentPhaseModel returns the model and fallback model for the current phase.
// Implements tools.PhaseModelProvider.
func (pr *PhaseRegistry) CurrentPhaseModel() (model string, fallback string) {
	phase, _, err := pr.phases.Current(pr.agentName)
	if err != nil || phase == nil {
		return "", ""
	}
	return phase.Model, phase.Fallback
}

// allowedNames returns a set of tool names allowed in the current phase,
// combined with teamTools and readTools. Returns nil if no phases are
// configured, signaling that all tools should be available.
func (pr *PhaseRegistry) allowedNames() map[string]bool {
	phase, _, err := pr.phases.Current(pr.agentName)
	if err != nil || phase == nil {
		return nil
	}
	allowed := make(map[string]bool)
	for _, name := range phase.Tools {
		allowed[name] = true
	}
	for _, name := range pr.teamTools {
		allowed[name] = true
	}
	for _, name := range pr.readTools {
		allowed[name] = true
	}
	return allowed
}

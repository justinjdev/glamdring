package teams

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/justin/glamdring/pkg/tools"
)

// AdvancePhaseTool advances the current workflow phase for an agent.
type AdvancePhaseTool struct {
	Registry  *ManagerRegistry
	AgentName string
}

type advancePhaseInput struct {
	TeamName string `json:"team_name"`
}

func (AdvancePhaseTool) Name() string { return "AdvancePhase" }

func (AdvancePhaseTool) Description() string {
	return "Advance to the next workflow phase."
}

func (AdvancePhaseTool) Schema() json.RawMessage {
	schema := map[string]any{
		"type":     "object",
		"required": []string{"team_name"},
		"properties": map[string]any{
			"team_name": map[string]any{
				"type":        "string",
				"description": "Name of the team",
			},
		},
	}
	b, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Sprintf("BUG: failed to marshal schema: %v", err))
	}
	return json.RawMessage(b)
}

func (a AdvancePhaseTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in advancePhaseInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tools.Result{Output: fmt.Sprintf("invalid input: %s", err), IsError: true}, nil
	}

	mgr, errResult := getManager(a.Registry, in.TeamName)
	if errResult != nil {
		return *errResult, nil
	}

	phase, err := mgr.Phases.Advance(a.AgentName)
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to advance phase: %s", err), IsError: true}, nil
	}

	out, err := json.Marshal(map[string]any{
		"phase_name": phase.Name,
		"tools":      phase.Tools,
		"model":      phase.Model,
	})
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to marshal phase result: %s", err), IsError: true}, nil
	}
	return tools.Result{Output: string(out)}, nil
}

package teams

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/justin/glamdring/pkg/tools"
)

// getManager retrieves a TeamManager from the registry with proper error handling.
func getManager(registry *ManagerRegistry, teamName string) (*TeamManager, *tools.Result) {
	if teamName == "" {
		return nil, &tools.Result{Output: "team_name is required", IsError: true}
	}
	mgr := registry.Get(teamName)
	if mgr == nil {
		return nil, &tools.Result{Output: fmt.Sprintf("team %q not found", teamName), IsError: true}
	}
	return mgr, nil
}

// TeamCreateTool creates a new team and registers it in the ManagerRegistry.
type TeamCreateTool struct {
	Registry *ManagerRegistry
	// TaskDirBase overrides the default task directory base (~/.glamdring/teams/).
	// When set, task files are stored under TaskDirBase/<team>/tasks/ instead.
	// Used by tests to avoid writing to the real home directory.
	TaskDirBase string
	// RegisteredWorkflows holds user-defined workflows from settings.
	// These are checked by ResolveWorkflow before built-in presets.
	RegisteredWorkflows map[string][]Phase
}

type teamCreateInput struct {
	TeamName    string `json:"team_name"`
	Description string `json:"description"`
	Workflow    string `json:"workflow"`
}

func (TeamCreateTool) Name() string { return "TeamCreate" }

func (TeamCreateTool) Description() string {
	return "Create a new agent team for coordinated multi-agent work."
}

func (TeamCreateTool) Schema() json.RawMessage {
	schema := map[string]any{
		"type":     "object",
		"required": []string{"team_name"},
		"properties": map[string]any{
			"team_name": map[string]any{
				"type":        "string",
				"description": "Unique name for the team",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Description of the team's purpose",
			},
			"workflow": map[string]any{
				"type":        "string",
				"description": "Built-in workflow name (rpiv, plan-implement, scoped, none)",
			},
		},
	}
	b, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Sprintf("BUG: failed to marshal schema: %v", err))
	}
	return json.RawMessage(b)
}

// validTeamName matches alphanumeric characters, hyphens, and underscores.
var validTeamName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

func (t TeamCreateTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in teamCreateInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tools.Result{Output: fmt.Sprintf("invalid input: %s", err), IsError: true}, nil
	}
	if in.TeamName == "" {
		return tools.Result{Output: "team_name is required", IsError: true}, nil
	}
	if !validTeamName.MatchString(in.TeamName) {
		return tools.Result{Output: "team_name must contain only alphanumeric characters, hyphens, and underscores", IsError: true}, nil
	}

	phases, err := ResolveWorkflow(in.Workflow, nil, t.RegisteredWorkflows)
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("unknown workflow %q", in.Workflow), IsError: true}, nil
	}
	cfg := TeamConfig{
		Name:        in.TeamName,
		Description: in.Description,
		Workflow:    in.Workflow,
		Phases:      phases,
	}

	var taskDir string
	if t.TaskDirBase != "" {
		taskDir = filepath.Join(t.TaskDirBase, in.TeamName, "tasks")
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return tools.Result{Output: fmt.Sprintf("failed to resolve home directory: %s", err), IsError: true}, nil
		}
		taskDir = filepath.Join(homeDir, ".glamdring", "teams", in.TeamName, "tasks")
	}

	_, err = t.Registry.Create(cfg, taskDir)
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to create team: %s", err), IsError: true}, nil
	}

	out, err := json.Marshal(map[string]string{
		"team_name": in.TeamName,
		"message":   fmt.Sprintf("team %q created", in.TeamName),
	})
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to marshal result: %s", err), IsError: true}, nil
	}
	return tools.Result{Output: string(out)}, nil
}

// TeamDeleteTool deletes an existing team from the ManagerRegistry.
type TeamDeleteTool struct {
	Registry *ManagerRegistry
}

type teamDeleteInput struct {
	TeamName string `json:"team_name"`
}

func (TeamDeleteTool) Name() string { return "TeamDelete" }

func (TeamDeleteTool) Description() string {
	return "Delete an agent team. The team must have no active members."
}

func (TeamDeleteTool) Schema() json.RawMessage {
	schema := map[string]any{
		"type":     "object",
		"required": []string{"team_name"},
		"properties": map[string]any{
			"team_name": map[string]any{
				"type":        "string",
				"description": "Name of the team to delete",
			},
		},
	}
	b, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Sprintf("BUG: failed to marshal schema: %v", err))
	}
	return json.RawMessage(b)
}

func (t TeamDeleteTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in teamDeleteInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tools.Result{Output: fmt.Sprintf("invalid input: %s", err), IsError: true}, nil
	}
	if in.TeamName == "" {
		return tools.Result{Output: "team_name is required", IsError: true}, nil
	}

	if err := t.Registry.Delete(in.TeamName); err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to delete team: %s", err), IsError: true}, nil
	}

	out, err := json.Marshal(map[string]string{
		"team_name": in.TeamName,
		"message":   fmt.Sprintf("team %q deleted", in.TeamName),
	})
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to marshal result: %s", err), IsError: true}, nil
	}
	return tools.Result{Output: string(out)}, nil
}

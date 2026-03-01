package teams

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/justin/glamdring/pkg/tools"
)

// TeamStatusTool returns a structured snapshot of team state including
// members, locks, task summary, and phase information per agent.
type TeamStatusTool struct {
	Registry *ManagerRegistry
}

type teamStatusInput struct {
	TeamName string `json:"team_name"`
}

func (TeamStatusTool) Name() string { return "TeamStatus" }

func (TeamStatusTool) Description() string {
	return "Get a snapshot of team state: members, locks, tasks, and phases."
}

func (TeamStatusTool) Schema() json.RawMessage {
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

type teamStatusOutput struct {
	TeamName string              `json:"team_name"`
	Members  []memberStatus      `json:"members"`
	Locks    []lockStatus        `json:"locks"`
	Tasks    taskStatusSummary   `json:"tasks"`
	Phases   []agentPhaseStatus  `json:"phases,omitempty"`
}

type memberStatus struct {
	Name   string       `json:"name"`
	Status MemberStatus `json:"status"`
}

type lockStatus struct {
	Path   string `json:"path"`
	Owner  string `json:"owner"`
	TaskID string `json:"task_id,omitempty"`
}

type taskStatusSummary struct {
	Total      int `json:"total"`
	Pending    int `json:"pending"`
	InProgress int `json:"in_progress"`
	Completed  int `json:"completed"`
}

type agentPhaseStatus struct {
	Agent string `json:"agent"`
	Phase string `json:"phase"`
	Index int    `json:"index"`
}

func (t TeamStatusTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in teamStatusInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tools.Result{Output: fmt.Sprintf("invalid input: %s", err), IsError: true}, nil
	}

	mgr, errResult := getManager(t.Registry, in.TeamName)
	if errResult != nil {
		return *errResult, nil
	}

	// Members.
	members := mgr.Members.List()
	memberOut := make([]memberStatus, len(members))
	for i, m := range members {
		memberOut[i] = memberStatus{Name: m.Name, Status: m.Status}
	}

	// Locks.
	lockMap := mgr.Locks.ListLocks()
	lockOut := make([]lockStatus, 0, len(lockMap))
	for path, entry := range lockMap {
		lockOut = append(lockOut, lockStatus{
			Path:   path,
			Owner:  entry.Owner,
			TaskID: entry.TaskID,
		})
	}

	// Tasks.
	tasks := mgr.Tasks.List()
	var summary taskStatusSummary
	summary.Total = len(tasks)
	for _, ts := range tasks {
		switch ts.Status {
		case TaskStatusPending:
			summary.Pending++
		case TaskStatusInProgress:
			summary.InProgress++
		case TaskStatusCompleted:
			summary.Completed++
		}
	}

	// Phases -- collect from all known members.
	var phaseOut []agentPhaseStatus
	for _, m := range members {
		phase, idx, err := mgr.Phases.Current(m.Name)
		if err != nil || phase == nil {
			continue
		}
		phaseOut = append(phaseOut, agentPhaseStatus{
			Agent: m.Name,
			Phase: phase.Name,
			Index: idx,
		})
	}

	out, err := json.Marshal(teamStatusOutput{
		TeamName: in.TeamName,
		Members:  memberOut,
		Locks:    lockOut,
		Tasks:    summary,
		Phases:   phaseOut,
	})
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to marshal status: %s", err), IsError: true}, nil
	}
	return tools.Result{Output: string(out)}, nil
}

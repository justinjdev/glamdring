package teams

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/justin/glamdring/pkg/tools"
)

// TaskCreateTool creates a new task in a team's task storage.
type TaskCreateTool struct {
	Registry *ManagerRegistry
}

type taskCreateInput struct {
	TeamName    string `json:"team_name"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
}

func (TaskCreateTool) Name() string { return "TaskCreate" }

func (TaskCreateTool) Description() string {
	return "Create a new task in a team's task board."
}

func (TaskCreateTool) Schema() json.RawMessage {
	schema := map[string]any{
		"type":     "object",
		"required": []string{"team_name", "subject"},
		"properties": map[string]any{
			"team_name": map[string]any{
				"type":        "string",
				"description": "Name of the team",
			},
			"subject": map[string]any{
				"type":        "string",
				"description": "Brief summary of the task",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Detailed task description",
			},
			"owner": map[string]any{
				"type":        "string",
				"description": "Agent name to assign as owner",
			},
		},
	}
	b, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Sprintf("BUG: failed to marshal schema: %v", err))
	}
	return json.RawMessage(b)
}

func (t TaskCreateTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in taskCreateInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tools.Result{Output: fmt.Sprintf("invalid input: %s", err), IsError: true}, nil
	}

	mgr, errResult := getManager(t.Registry, in.TeamName)
	if errResult != nil {
		return *errResult, nil
	}

	if in.Subject == "" {
		return tools.Result{Output: "subject is required", IsError: true}, nil
	}

	task := Task{
		Subject:     in.Subject,
		Description: in.Description,
		Owner:       in.Owner,
		Status:      TaskStatusPending,
	}

	created, err := mgr.Tasks.Create(task)
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to create task: %s", err), IsError: true}, nil
	}

	out, err := json.Marshal(map[string]string{
		"task_id": created.ID,
		"subject": created.Subject,
	})
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to marshal result: %s", err), IsError: true}, nil
	}
	return tools.Result{Output: string(out)}, nil
}

// TaskListTool lists all tasks in a team.
type TaskListTool struct {
	Registry *ManagerRegistry
}

type taskListInput struct {
	TeamName string `json:"team_name"`
}

func (TaskListTool) Name() string { return "TaskList" }

func (TaskListTool) Description() string {
	return "List all tasks in a team's task board."
}

func (TaskListTool) Schema() json.RawMessage {
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

func (t TaskListTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in taskListInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tools.Result{Output: fmt.Sprintf("invalid input: %s", err), IsError: true}, nil
	}

	mgr, errResult := getManager(t.Registry, in.TeamName)
	if errResult != nil {
		return *errResult, nil
	}

	summaries := mgr.Tasks.List()
	out, err := json.Marshal(summaries)
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to marshal task list: %s", err), IsError: true}, nil
	}
	return tools.Result{Output: string(out)}, nil
}

// TaskGetTool retrieves a single task by ID.
type TaskGetTool struct {
	Registry *ManagerRegistry
}

type taskGetInput struct {
	TeamName string `json:"team_name"`
	TaskID   string `json:"task_id"`
}

func (TaskGetTool) Name() string { return "TaskGet" }

func (TaskGetTool) Description() string {
	return "Get the full details of a task by its ID."
}

func (TaskGetTool) Schema() json.RawMessage {
	schema := map[string]any{
		"type":     "object",
		"required": []string{"team_name", "task_id"},
		"properties": map[string]any{
			"team_name": map[string]any{
				"type":        "string",
				"description": "Name of the team",
			},
			"task_id": map[string]any{
				"type":        "string",
				"description": "ID of the task to retrieve",
			},
		},
	}
	b, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Sprintf("BUG: failed to marshal schema: %v", err))
	}
	return json.RawMessage(b)
}

func (t TaskGetTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in taskGetInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tools.Result{Output: fmt.Sprintf("invalid input: %s", err), IsError: true}, nil
	}

	mgr, errResult := getManager(t.Registry, in.TeamName)
	if errResult != nil {
		return *errResult, nil
	}

	if in.TaskID == "" {
		return tools.Result{Output: "task_id is required", IsError: true}, nil
	}

	task, err := mgr.Tasks.Get(in.TaskID)
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to get task: %s", err), IsError: true}, nil
	}

	out, err := json.Marshal(task)
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to marshal task: %s", err), IsError: true}, nil
	}
	return tools.Result{Output: string(out)}, nil
}

// TaskUpdateTool updates an existing task's fields.
type TaskUpdateTool struct {
	Registry  *ManagerRegistry
	AgentName string // trusted identity for checkin reset
}

type taskUpdateInput struct {
	TeamName      string   `json:"team_name"`
	TaskID        string   `json:"task_id"`
	Status        string   `json:"status"`
	Subject       string   `json:"subject"`
	Description   string   `json:"description"`
	Owner         string   `json:"owner"`
	ExpectedOwner string   `json:"expected_owner"`
	AddBlocks     []string `json:"add_blocks"`
	AddBlockedBy  []string `json:"add_blocked_by"`
}

func (TaskUpdateTool) Name() string { return "TaskUpdate" }

func (TaskUpdateTool) Description() string {
	return "Update a task's status, owner, description, or dependency fields."
}

func (TaskUpdateTool) Schema() json.RawMessage {
	schema := map[string]any{
		"type":     "object",
		"required": []string{"team_name", "task_id"},
		"properties": map[string]any{
			"team_name": map[string]any{
				"type":        "string",
				"description": "Name of the team",
			},
			"task_id": map[string]any{
				"type":        "string",
				"description": "ID of the task to update",
			},
			"status": map[string]any{
				"type":        "string",
				"description": "New status (pending, in_progress, completed, deleted)",
			},
			"subject": map[string]any{
				"type":        "string",
				"description": "New subject line",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "New description",
			},
			"owner": map[string]any{
				"type":        "string",
				"description": "New owner agent name",
			},
			"expected_owner": map[string]any{
				"type":        "string",
				"description": "Expected current owner for compare-and-swap",
			},
			"add_blocks": map[string]any{
				"type":        "array",
				"description": "Task IDs that this task blocks",
				"items":       map[string]any{"type": "string"},
			},
			"add_blocked_by": map[string]any{
				"type":        "array",
				"description": "Task IDs that block this task",
				"items":       map[string]any{"type": "string"},
			},
		},
	}
	b, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Sprintf("BUG: failed to marshal schema: %v", err))
	}
	return json.RawMessage(b)
}

func (t TaskUpdateTool) Execute(_ context.Context, input json.RawMessage) (tools.Result, error) {
	var in taskUpdateInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tools.Result{Output: fmt.Sprintf("invalid input: %s", err), IsError: true}, nil
	}

	mgr, errResult := getManager(t.Registry, in.TeamName)
	if errResult != nil {
		return *errResult, nil
	}

	if in.TaskID == "" {
		return tools.Result{Output: "task_id is required", IsError: true}, nil
	}

	update := TaskUpdate{
		AddBlocks:    in.AddBlocks,
		AddBlockedBy: in.AddBlockedBy,
	}

	if in.Status != "" {
		s := TaskStatus(in.Status)
		if !s.Valid() {
			return tools.Result{
				Output:  fmt.Sprintf("invalid status %q: must be one of pending, in_progress, completed, deleted", in.Status),
				IsError: true,
			}, nil
		}
		update.Status = &s
	}
	if in.Subject != "" {
		update.Subject = &in.Subject
	}
	if in.Description != "" {
		update.Description = &in.Description
	}
	if in.Owner != "" {
		update.Owner = &in.Owner
	}
	if in.ExpectedOwner != "" {
		update.ExpectedOwner = &in.ExpectedOwner
	}

	updated, err := mgr.Tasks.Update(in.TaskID, update)
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to update task: %s", err), IsError: true}, nil
	}

	if update.Status != nil && *update.Status == TaskStatusCompleted {
		mgr.Locks.ReleaseByTask(in.TaskID)
	}

	if t.AgentName != "" {
		mgr.Checkins.Reset(t.AgentName)
	}

	out, err := json.Marshal(updated)
	if err != nil {
		return tools.Result{Output: fmt.Sprintf("failed to marshal result: %s", err), IsError: true}, nil
	}
	return tools.Result{Output: string(out)}, nil
}

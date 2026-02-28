package teams

import "time"

// MemberStatus represents the current state of a team member.
type MemberStatus string

const (
	MemberStatusIdle     MemberStatus = "idle"
	MemberStatusActive   MemberStatus = "active"
	MemberStatusShutdown MemberStatus = "shutdown"
)

// Member represents a team agent.
type Member struct {
	Name      string       `json:"name"`
	AgentType string       `json:"agent_type,omitempty"`
	Status    MemberStatus `json:"status"`
	AgentID   string       `json:"agent_id,omitempty"`
}

// TaskStatus represents task lifecycle states.
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusDeleted    TaskStatus = "deleted"
)

// Valid returns true if the status is one of the known task statuses.
func (s TaskStatus) Valid() bool {
	switch s {
	case TaskStatusPending, TaskStatusInProgress, TaskStatusCompleted, TaskStatusDeleted:
		return true
	default:
		return false
	}
}

// TaskScope defines file path patterns an agent may modify.
type TaskScope struct {
	AllowPatterns []string `json:"allow_patterns,omitempty"`
	DenyPatterns  []string `json:"deny_patterns,omitempty"`
}

// Task represents a unit of work in a team.
type Task struct {
	ID          string     `json:"id"`
	Subject     string     `json:"subject"`
	Description string     `json:"description,omitempty"`
	Status      TaskStatus `json:"status"`
	Owner       string     `json:"owner,omitempty"`
	Blocks      []string   `json:"blocks,omitempty"`
	BlockedBy   []string   `json:"blocked_by,omitempty"`
	Scope       *TaskScope `json:"scope,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TaskSummary is a lightweight view of a task for list operations.
type TaskSummary struct {
	ID        string     `json:"id"`
	Subject   string     `json:"subject"`
	Status    TaskStatus `json:"status"`
	Owner     string     `json:"owner,omitempty"`
	BlockedBy []string   `json:"blocked_by,omitempty"`
}

// TaskUpdate describes changes to apply to a task.
type TaskUpdate struct {
	Status        *TaskStatus `json:"status,omitempty"`
	Subject       *string     `json:"subject,omitempty"`
	Description   *string     `json:"description,omitempty"`
	Owner         *string     `json:"owner,omitempty"`
	ExpectedOwner *string     `json:"expected_owner,omitempty"` // CAS field
	AddBlocks     []string    `json:"add_blocks,omitempty"`
	AddBlockedBy  []string    `json:"add_blocked_by,omitempty"`
	Scope         *TaskScope  `json:"scope,omitempty"`
}

// MessageKind categorizes inter-agent messages.
type MessageKind string

const (
	MessageKindDM               MessageKind = "dm"
	MessageKindBroadcast        MessageKind = "broadcast"
	MessageKindShutdownRequest  MessageKind = "shutdown_request"
	MessageKindShutdownResponse MessageKind = "shutdown_response"
	MessageKindApprovalRequest  MessageKind = "approval_request"
	MessageKindApprovalResponse MessageKind = "approval_response"
)

// AgentMessage is a message between team agents.
type AgentMessage struct {
	Kind      MessageKind `json:"kind"`
	From      string      `json:"from"`
	To        string      `json:"to,omitempty"` // empty for broadcast
	Content   string      `json:"content"`
	RequestID string      `json:"request_id,omitempty"`
	Approve   *bool       `json:"approve,omitempty"`
	Priority  bool        `json:"priority,omitempty"`
}

// Phase defines a workflow stage with tool access and model configuration.
type Phase struct {
	Name     string   `json:"name"`
	Tools    []string `json:"tools"`
	Model    string   `json:"model,omitempty"`
	Fallback string   `json:"fallback,omitempty"`
}

// GateType defines how phase transitions are triggered.
type GateType string

const (
	GateTypeAuto      GateType = "auto"
	GateTypeLeader    GateType = "leader"
	GateTypeCondition GateType = "condition"
)

// TeamConfig holds the configuration for a team.
type TeamConfig struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Workflow    string            `json:"workflow,omitempty"`
	Phases      []Phase           `json:"phases,omitempty"`
	Members     []Member          `json:"members,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

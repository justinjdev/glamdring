package teams

import (
	"fmt"
	"time"
)

// MemberStatus represents the current state of a team member.
type MemberStatus string

const (
	MemberStatusIdle     MemberStatus = "idle"
	MemberStatusActive   MemberStatus = "active"
	MemberStatusShutdown MemberStatus = "shutdown"
)

// Valid returns true if the status is one of the known member statuses.
func (s MemberStatus) Valid() bool {
	switch s {
	case MemberStatusIdle, MemberStatusActive, MemberStatusShutdown:
		return true
	default:
		return false
	}
}

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

// Valid returns true if the kind is one of the known message kinds.
func (k MessageKind) Valid() bool {
	switch k {
	case MessageKindDM, MessageKindBroadcast, MessageKindShutdownRequest,
		MessageKindShutdownResponse, MessageKindApprovalRequest, MessageKindApprovalResponse:
		return true
	default:
		return false
	}
}

// AgentMessage is a message between team agents.
type AgentMessage struct {
	Kind      MessageKind `json:"kind"`
	From      string      `json:"from"`
	To        string      `json:"to,omitempty"` // empty for broadcast
	Content   string      `json:"content"`
	RequestID string      `json:"request_id,omitempty"`
	Approve   *bool       `json:"approve,omitempty"`
	Force     bool        `json:"force,omitempty"`
	Timestamp time.Time   `json:"timestamp,omitempty"`
	SeqNum    int         `json:"seq_num,omitempty"`
}

// Validate checks that the message has the required fields for its Kind.
func (m AgentMessage) Validate() error {
	if !m.Kind.Valid() {
		return fmt.Errorf("invalid message kind %q", m.Kind)
	}
	if m.From == "" {
		return fmt.Errorf("from is required")
	}
	switch m.Kind {
	case MessageKindDM:
		if m.To == "" {
			return fmt.Errorf("dm requires To")
		}
		if m.Content == "" {
			return fmt.Errorf("dm requires Content")
		}
	case MessageKindBroadcast:
		if m.To != "" {
			return fmt.Errorf("broadcast must not set To")
		}
	case MessageKindShutdownRequest:
		if m.To == "" {
			return fmt.Errorf("shutdown_request requires To")
		}
	case MessageKindShutdownResponse:
		if m.RequestID == "" {
			return fmt.Errorf("shutdown_response requires RequestID")
		}
		if m.Approve == nil {
			return fmt.Errorf("shutdown_response requires Approve")
		}
	case MessageKindApprovalRequest:
		if m.To == "" {
			return fmt.Errorf("approval_request requires To")
		}
	case MessageKindApprovalResponse:
		if m.To == "" {
			return fmt.Errorf("approval_response requires To")
		}
		if m.RequestID == "" {
			return fmt.Errorf("approval_response requires RequestID")
		}
		if m.Approve == nil {
			return fmt.Errorf("approval_response requires Approve")
		}
	}
	return nil
}

// GateType controls how phase transitions are enforced.
type GateType string

const (
	GateAuto      GateType = "auto"
	GateLeader    GateType = "leader"
	GateCondition GateType = "condition"
)

// Valid returns true if the gate type is one of the known gate types.
// Empty string is valid and treated as GateAuto.
func (g GateType) Valid() bool {
	switch g {
	case "", GateAuto, GateLeader, GateCondition:
		return true
	default:
		return false
	}
}

// Phase defines a workflow stage with tool access and model configuration.
type Phase struct {
	Name       string            `json:"name"`
	Tools      []string          `json:"tools"`
	Model      string            `json:"model,omitempty"`
	Fallback   string            `json:"fallback,omitempty"`
	Gate       GateType          `json:"gate,omitempty"`
	GateConfig map[string]string `json:"gate_config,omitempty"`
}

// TeamConfig holds the configuration for a team.
type TeamConfig struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Workflow    string            `json:"workflow,omitempty"`
	Phases      []Phase           `json:"phases,omitempty"`
	Members     []Member          `json:"members,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Leader      string            `json:"leader,omitempty"`
}

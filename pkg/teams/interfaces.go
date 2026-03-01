package teams

// MemberRegistry tracks team members and their status.
type MemberRegistry interface {
	Add(member Member) error
	Remove(name string) error
	Get(name string) (*Member, error)
	SetStatus(name string, status MemberStatus) error
	List() []Member
	Count() int
	ActiveCount() int
	FirstMember() (string, bool)
}

// TaskStorage manages task persistence and querying.
type TaskStorage interface {
	Create(task Task) (*Task, error)
	Get(id string) (*Task, error)
	Update(id string, update TaskUpdate) (*Task, error)
	List() []TaskSummary
	Delete(id string) error
}

// MessageTransport handles inter-agent message delivery.
type MessageTransport interface {
	Send(msg AgentMessage) error
	Subscribe(agentName string, bufferSize int) (<-chan AgentMessage, <-chan AgentMessage, error) // regular, priority
	Unsubscribe(agentName string)
}

// LockManager provides file-level locking for agents.
type LockManager interface {
	Acquire(path string, owner string) error
	AcquireForTask(path, owner, taskID string) error
	Release(path string, owner string) error
	ReleaseByTask(taskID string)
	Check(path string) (owner string, locked bool)
	ReleaseAll(owner string)
	ListLocks() map[string]LockEntry
}

// ContextCache stores summarized context for phase transitions.
type ContextCache interface {
	Store(key string, value string)
	Load(key string) (string, bool)
	Delete(key string)
}

// PhaseTracker tracks workflow phase progression per agent.
type PhaseTracker interface {
	SetPhases(agentName string, phases []Phase)
	Current(agentName string) (*Phase, int, error) // phase, index, error
	Advance(agentName string) (*Phase, error)      // returns new phase
	AdvanceTo(agentName string, phaseName string) (*Phase, error)
	Remove(agentName string)
}

// CheckinTracker monitors agent activity via tool call counting.
type CheckinTracker interface {
	Increment(agentName string) int
	Reset(agentName string)
	Count(agentName string) int
	Remove(agentName string)
}

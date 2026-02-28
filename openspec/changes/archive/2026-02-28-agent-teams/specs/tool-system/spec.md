## ADDED Requirements

### Requirement: Tool decorator interface
The system SHALL support a tool decorator pattern where a wrapper implements the Tool interface and delegates to an inner tool after performing checks. Decorators SHALL be composable -- multiple decorators can wrap the same base tool in a chain.

#### Scenario: Single decorator wrapping
- **WHEN** a ScopedEdit decorator wraps the base Edit tool
- **THEN** ScopedEdit.Execute() checks path scope and, if valid, calls Edit.Execute()

#### Scenario: Multi-decorator chain
- **WHEN** CheckinGate wraps FileLock wraps ScopedEdit wraps Edit
- **THEN** a call to the outermost Execute() traverses all decorators in order before reaching Edit.Execute()

#### Scenario: Decorator preserves tool identity
- **WHEN** a ScopedEdit decorator wraps Edit
- **THEN** ScopedEdit.Name() returns "Edit" and ScopedEdit.Schema() returns the same schema as Edit, so the model sees the same tool interface

### Requirement: PhaseRegistry for dynamic tool filtering
The system SHALL provide a PhaseRegistry that wraps the standard Registry and filters tools based on a current phase configuration. PhaseRegistry SHALL implement the same Schemas() and Get() methods as Registry. Tools not in the current phase's whitelist SHALL be excluded from Schemas() and return nil from Get().

#### Scenario: Schemas returns only phase tools
- **WHEN** PhaseRegistry.Schemas() is called and the current phase allows [Read, Glob, Grep, SendMessage, TaskUpdate, AdvancePhase]
- **THEN** only those 6 tools appear in the returned schema array

#### Scenario: Get returns nil for excluded tool
- **WHEN** PhaseRegistry.Get("Edit") is called and Edit is not in the current phase
- **THEN** nil is returned

#### Scenario: Phase change updates available tools
- **WHEN** the phase advances from "research" to "implement"
- **THEN** subsequent calls to Schemas() and Get() reflect the new phase's tool whitelist

### Requirement: Team tools depend on interfaces, not implementations
Team tools (SendMessage, TaskCreate, TaskList, TaskGet, TaskUpdate) SHALL depend on the MessageTransport and TaskStorage interfaces defined in `pkg/teams/`, not on concrete implementations. This enables testing with mock transports and future replacement with distributed backends without changing tool code.

#### Scenario: SendMessage uses MessageTransport interface
- **WHEN** the SendMessage tool delivers a message
- **THEN** it calls MessageTransport.Send(), not a concrete channel write

#### Scenario: TaskCreate uses TaskStorage interface
- **WHEN** the TaskCreate tool persists a task
- **THEN** it calls TaskStorage.Create(), not a direct file write

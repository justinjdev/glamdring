## ADDED Requirements

### Requirement: Create tasks with scope metadata
The system SHALL provide a TaskCreate tool that creates a task with: subject, description, status (pending), and optional scope metadata. Scope metadata defines file path patterns and command patterns that constrain what the assigned agent can modify. Tasks SHALL be persisted as JSON files in the team's task directory.

#### Scenario: Create task with file scope
- **WHEN** TaskCreate is called with subject "Implement auth middleware" and scope paths ["pkg/auth/**", "pkg/middleware/**"]
- **THEN** a task is created with status "pending" and the scope is stored, ready to be enforced when an agent claims the task

#### Scenario: Create task without scope
- **WHEN** TaskCreate is called without scope metadata
- **THEN** the task is created with no scope restrictions (agent can access any file)

### Requirement: List tasks with summary view
The system SHALL provide a TaskList tool that returns all tasks in the team's task list with: id, subject, status, owner, and blockedBy fields. Completed and deleted tasks SHALL be included with their status.

#### Scenario: List tasks showing blocked status
- **WHEN** TaskList is called and task 2 is blocked by task 1
- **THEN** the output shows task 2 with blockedBy: ["1"] so agents know not to claim it

### Requirement: Get full task details
The system SHALL provide a TaskGet tool that returns full task details by ID, including: subject, description, status, owner, scope, dependencies (blocks/blockedBy), turn budget, and turns used.

#### Scenario: Get task with scope and budget
- **WHEN** TaskGet is called for a task with scope and a 20-turn budget
- **THEN** the output includes the full scope definition and shows "turnsUsed: 5 / maxTurns: 20"

### Requirement: Update task status and ownership
The system SHALL provide a TaskUpdate tool that can update: status (pending, in_progress, completed, deleted), owner, subject, description, and dependencies (addBlocks, addBlockedBy).

#### Scenario: Agent claims a task
- **WHEN** an agent calls TaskUpdate with taskId "3" and owner "auth-impl"
- **THEN** the task owner is set to "auth-impl"

#### Scenario: Mark task completed
- **WHEN** an agent calls TaskUpdate with taskId "3" and status "completed"
- **THEN** the task status changes and any tasks blocked by task 3 have their blockedBy list updated

### Requirement: Atomic task claiming with compare-and-swap
When an agent sets itself as owner of a task via TaskUpdate, the operation SHALL use compare-and-swap semantics. TaskUpdate with an `owner` field SHALL accept an optional `expected_owner` field (defaulting to empty string, meaning "currently unowned"). If the task's current owner does not match `expected_owner`, the update SHALL fail with an error identifying the current owner. This prevents race conditions where two agents read TaskList, both see a task as unowned, and both attempt to claim it.

#### Scenario: Successful claim of unowned task
- **WHEN** agent "auth-impl" calls TaskUpdate with taskId "3", owner "auth-impl", and expected_owner "" (or omitted)
- **THEN** the task's current owner is "" (unowned), so the claim succeeds and owner is set to "auth-impl"

#### Scenario: Claim rejected due to race
- **WHEN** agent "api-impl" calls TaskUpdate with taskId "3", owner "api-impl", and expected_owner ""
- **AND** agent "auth-impl" has already claimed task 3
- **THEN** the tool returns an error: "Task 3 is owned by 'auth-impl' (expected: unowned). Check TaskList for available tasks."

#### Scenario: Reassignment by lead
- **WHEN** the lead calls TaskUpdate with taskId "3", owner "api-impl", and expected_owner "auth-impl"
- **THEN** the reassignment succeeds because the expected owner matches the current owner

### Requirement: Task dependencies
Tasks SHALL support dependency relationships: a task can block other tasks, and a task can be blocked by other tasks. A blocked task (non-empty blockedBy list of non-completed tasks) SHALL NOT be claimable.

#### Scenario: Blocked task cannot be claimed
- **WHEN** an agent tries to set itself as owner of a task that is blocked by an incomplete task
- **THEN** the tool returns an error indicating which tasks must complete first

### Requirement: Turn budgets on tasks
Tasks SHALL support an optional maxTurns field. When an agent is working on a task with a turn budget, each completed turn increments turnsUsed. When turnsUsed reaches maxTurns, the agent's next non-read tool call SHALL return an error directing the agent to report status and stop.

#### Scenario: Turn budget exhausted
- **WHEN** an agent has used 20 of 20 turns on a task and calls Edit
- **THEN** the Edit tool returns an error: "Turn budget exhausted (20/20). Report status via SendMessage and TaskUpdate before continuing."

#### Scenario: Read tools still work after budget exhaustion
- **WHEN** an agent has exhausted its turn budget and calls Read
- **THEN** the Read tool executes normally (read-only tools are never budget-restricted)

## ADDED Requirements

### Requirement: Direct messages between agents
The system SHALL provide a SendMessage tool that delivers a message from one agent to another by name. Messages SHALL be delivered between turns -- the recipient sees the message as an injected user-role message at the start of its next turn iteration. Each message SHALL include sender name, content, and a short summary.

#### Scenario: Agent sends direct message
- **WHEN** agent "auth-impl" calls SendMessage with recipient "team-lead" and content "Finished research phase, found 3 relevant files"
- **THEN** the message is placed in team-lead's mailbox and delivered at the start of team-lead's next turn

#### Scenario: Message to nonexistent agent
- **WHEN** SendMessage is called with a recipient name that doesn't match any team member
- **THEN** the tool returns an error listing available team members

### Requirement: Broadcast messages
The system SHALL support broadcast messages that are delivered to all team members except the sender. Broadcasts SHALL be used sparingly -- the tool description SHALL warn about cost scaling.

#### Scenario: Lead broadcasts to all agents
- **WHEN** the lead calls SendMessage with type "broadcast" and 3 active team members
- **THEN** all 3 members receive the message in their mailboxes

### Requirement: Shutdown request and response
The system SHALL support a shutdown protocol via SendMessage. A shutdown request includes type "shutdown_request" and a target recipient. The recipient SHALL receive the request and MUST respond via SendMessage with type "shutdown_response" containing approve (true/false) and optional content.

#### Scenario: Shutdown approved
- **WHEN** the lead sends a shutdown_request to "auth-impl" and auth-impl responds with approve: true
- **THEN** auth-impl's goroutine terminates and the lead receives confirmation

#### Scenario: Shutdown rejected
- **WHEN** the lead sends a shutdown_request and the agent responds with approve: false and content "Still working"
- **THEN** the agent continues running and the lead receives the rejection with the reason

### Requirement: Phase approval flow
The system SHALL support phase approval via messaging. When an agent's AdvancePhase tool blocks on a LeaderApproval gate, a phase approval request is sent to the team lead. The lead SHALL respond via SendMessage with type "plan_approval_response" containing approve (true/false) and optional feedback.

#### Scenario: Leader approves phase advance
- **WHEN** agent "auth-impl" calls AdvancePhase, hits a LeaderApproval gate, and the lead approves
- **THEN** the AdvancePhase tool unblocks, the agent advances to the next phase, and its available tools change

#### Scenario: Leader rejects phase advance
- **WHEN** the lead responds with approve: false and content "Plan doesn't cover error handling"
- **THEN** the AdvancePhase tool returns an error with the feedback, and the agent remains in the current phase

### Requirement: Message delivery to idle agents
Messages sent to an idle agent (waiting between turns) SHALL wake the agent. The agent's goroutine resumes and processes the message as a new turn.

#### Scenario: Wake idle agent
- **WHEN** agent "auth-impl" is idle (completed its last turn) and the lead sends it a message
- **THEN** the agent wakes, receives the message, and begins a new turn

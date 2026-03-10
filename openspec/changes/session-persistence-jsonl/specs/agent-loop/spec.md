## ADDED Requirements

### Requirement: Persist messages after each turn
When a session store is configured, the agent loop SHALL append all new messages to the session's JSONL file after each turn completes. Persistence failures SHALL be logged but SHALL NOT interrupt the agent loop.

#### Scenario: Messages persisted after turn
- **WHEN** an agent turn completes (assistant response appended to messages)
- **THEN** all messages since the last save are appended to the JSONL file and `lastSavedIndex` is advanced

#### Scenario: Persistence error does not abort loop
- **WHEN** a JSONL write fails (e.g., disk full)
- **THEN** the error is logged and the agent loop continues normally

#### Scenario: No store configured
- **WHEN** the session store is nil (persistence disabled or not configured)
- **THEN** no write is attempted and loop behavior is unchanged

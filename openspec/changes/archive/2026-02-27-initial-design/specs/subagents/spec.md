## ADDED Requirements

### Requirement: Spawn subagent tasks
The system SHALL support spawning subagents — independent agent loops that run concurrently as goroutines. Each subagent gets its own conversation context, system prompt, and tool set. The parent agent communicates with subagents via the Task tool.

#### Scenario: Spawn and receive result
- **WHEN** the parent agent calls the Task tool with a prompt and subagent type
- **THEN** a new agent loop starts in a goroutine, executes the task, and returns the result to the parent

### Requirement: Subagent isolation
Each subagent SHALL have its own message history and API conversation. Subagents SHALL share the filesystem with the parent agent (no working directory isolation). Subagents SHALL respect the same permission model as the parent.

#### Scenario: Subagent reads file modified by parent
- **WHEN** the parent modifies a file and then a subagent reads it
- **THEN** the subagent sees the modified content (shared filesystem)

### Requirement: Subagent tool restrictions
Subagents SHALL accept a tool whitelist. If specified, the subagent can only use the listed tools. If not specified, the subagent inherits the parent's tool set.

#### Scenario: Read-only subagent
- **WHEN** a subagent is spawned with tools restricted to `[Read, Glob, Grep]`
- **THEN** the subagent cannot call Write, Edit, or Bash

### Requirement: Concurrent subagent execution
Multiple subagents SHALL be able to run concurrently. The parent agent SHALL be able to spawn multiple tasks and receive results as they complete.

#### Scenario: Parallel tasks
- **WHEN** the parent agent spawns three subagents simultaneously
- **THEN** all three run concurrently and results are returned as each completes

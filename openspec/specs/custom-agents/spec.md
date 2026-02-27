# custom-agents Specification

## Purpose
TBD - created by archiving change initial-design. Update Purpose after archive.
## Requirements
### Requirement: Load custom agent definitions
The system SHALL scan `.claude/agents/` in both project-level and user-level locations for agent definition files (markdown or YAML). Each file defines a custom agent with a name, description, system prompt, and allowed tools list.

#### Scenario: Project-level agent
- **WHEN** `.claude/agents/code-reviewer.md` exists with a description and tool list
- **THEN** `code-reviewer` is available as a subagent type when spawning tasks

### Requirement: Custom agents available to Task tool
Custom agents SHALL be available as subagent types when the agent loop spawns subagents via the Task tool. The custom agent's system prompt and tool restrictions SHALL override the defaults for that subagent.

#### Scenario: Spawn custom agent
- **WHEN** the agent requests a Task with `subagent_type: "code-reviewer"`
- **THEN** the subagent runs with the code-reviewer's defined system prompt and tool set

### Requirement: Agent definition format
Agent definitions SHALL include: `name`, `description`, `prompt` (system prompt content), and `tools` (list of allowed tool names). The format SHALL be compatible with Claude Code's `.claude/agents/` format.

#### Scenario: Agent with restricted tools
- **WHEN** an agent definition specifies `tools: [Read, Glob, Grep]`
- **THEN** the subagent can only use those three tools, regardless of what tools are available to the parent agent


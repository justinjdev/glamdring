## MODIFIED Requirements

### Requirement: Load custom agent definitions
The system SHALL scan agent directories in both project-level and user-level locations for agent definition files (markdown or YAML). At each level, the system SHALL check `.glamdring/agents/` first and `.claude/agents/` as fallback (using centralized directory resolution). Each file defines a custom agent with a name, description, system prompt, and allowed tools list.

#### Scenario: Project-level agent in .glamdring/
- **WHEN** `.glamdring/agents/code-reviewer.md` exists with a description and tool list
- **THEN** `code-reviewer` is available as a subagent type when spawning tasks

#### Scenario: Project-level agent fallback to .claude/
- **WHEN** `.glamdring/agents/` does not exist and `.claude/agents/code-reviewer.md` exists
- **THEN** `code-reviewer` is available as a subagent type

#### Scenario: .glamdring/ agents take precedence
- **WHEN** both `.glamdring/agents/code-reviewer.md` and `.claude/agents/code-reviewer.md` exist
- **THEN** the agent definition SHALL be loaded from `.glamdring/agents/code-reviewer.md`

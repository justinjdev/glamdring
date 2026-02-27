# built-in-commands Specification

## Purpose
TBD - created by archiving change built-in-commands. Update Purpose after archive.
## Requirements
### Requirement: Built-in command dispatch
The TUI SHALL intercept slash commands matching built-in names before checking the user-defined command registry. Built-in commands SHALL execute locally without invoking the agent.

#### Scenario: Built-in command takes precedence
- **WHEN** user types a slash command that matches both a built-in and a user-defined command
- **THEN** the built-in command executes and the user-defined command is not invoked

#### Scenario: Unknown command falls through
- **WHEN** user types a slash command that is not a built-in
- **THEN** the system checks the user-defined command registry as before

### Requirement: /help command
The system SHALL display a list of all available commands when the user types `/help`. The list SHALL include both built-in and user-defined commands with brief descriptions.

#### Scenario: Display help
- **WHEN** user types `/help`
- **THEN** system displays all built-in commands with descriptions, followed by any user-defined commands

### Requirement: /clear command
The system SHALL clear the output viewport and reset the token/turn counters in the statusbar when the user types `/clear`. Conversation history for the agent is not affected.

#### Scenario: Clear output
- **WHEN** user types `/clear`
- **THEN** the output viewport is emptied, the statusbar token counters and turn counter reset to zero, and the input remains focused

### Requirement: /config command
The system SHALL display the current configuration when the user types `/config`. The display SHALL include the active model, max turns setting, and any configured MCP servers.

#### Scenario: Display config
- **WHEN** user types `/config`
- **THEN** system displays the current model name, max turns value, and list of configured MCP server names

### Requirement: /model command
The system SHALL switch the active model for subsequent agent turns when the user types `/model <name>`. The statusbar SHALL update immediately to reflect the new model.

#### Scenario: Switch model
- **WHEN** user types `/model claude-sonnet-4-6`
- **THEN** the agent config model is updated to `claude-sonnet-4-6` and the statusbar reflects the change

#### Scenario: No model name provided
- **WHEN** user types `/model` with no argument
- **THEN** system displays the current model name

### Requirement: /cost command
The system SHALL display cumulative token usage and estimated cost when the user types `/cost`.

#### Scenario: Display cost
- **WHEN** user types `/cost`
- **THEN** system displays total input tokens, total output tokens, estimated cost, and number of turns

### Requirement: /compact command
The system SHALL compress the conversation context when the user types `/compact`. The command SHALL send a summarization prompt to the agent that produces a structured context block, write the summary to a checkpoint file, and then truncate the conversation history to the summary.

#### Scenario: Compact conversation
- **WHEN** user types `/compact`
- **THEN** system sends a summarization prompt to the agent, displays the structured summary, writes the summary to `tmp/checkpoint.md` in the working directory with a timestamp header, and replaces the conversation history with the compact summary

#### Scenario: Checkpoint file format
- **WHEN** a compact summary is written to disk
- **THEN** the file contains a timestamp comment, git branch comment, and the structured summary with sections: Task, Key Findings, Files, Current State, and Next Steps

#### Scenario: Checkpoint directory creation
- **WHEN** the `tmp/` directory does not exist in the working directory
- **THEN** the system creates it before writing the checkpoint

### Requirement: /quit command
The system SHALL exit glamdring when the user types `/quit`.

#### Scenario: Quit
- **WHEN** user types `/quit`
- **THEN** the program exits cleanly with zero status

### Requirement: Tab completion for built-in commands
Built-in command names SHALL be included in the tab completion system alongside user-defined commands.

#### Scenario: Tab complete built-in
- **WHEN** user types `/he` and presses Tab
- **THEN** the input is completed to `/help`


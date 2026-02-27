# api-client Specification

## Purpose
TBD - created by archiving change initial-design. Update Purpose after archive.
## Requirements
### Requirement: Send messages to Claude API
The system SHALL send requests to `POST /v1/messages` with model, max_tokens, messages, system prompt, and tools parameters. The system SHALL authenticate using the `ANTHROPIC_API_KEY` environment variable via the `x-api-key` header.

#### Scenario: Basic message request
- **WHEN** the agent sends a prompt with no prior conversation history
- **THEN** the system sends a POST request to `/v1/messages` with the prompt as a single user message and receives a response

#### Scenario: Missing API key
- **WHEN** `ANTHROPIC_API_KEY` is not set
- **THEN** the system SHALL exit with a clear error message before making any API call

### Requirement: Stream responses via SSE
The system SHALL use server-sent events (SSE) streaming for all API requests by setting `stream: true`. The system SHALL parse `message_start`, `content_block_start`, `content_block_delta`, `content_block_stop`, `message_delta`, and `message_stop` events and emit them as typed messages on the output channel.

#### Scenario: Streaming text response
- **WHEN** the API streams text deltas
- **THEN** the system emits each `text_delta` as a `TextDelta` message on the output channel as it arrives

#### Scenario: Streaming tool use response
- **WHEN** the API streams a `tool_use` content block
- **THEN** the system accumulates `input_json_delta` events and emits a complete `ToolCall` message when `content_block_stop` is received

### Requirement: Support adaptive thinking
The system SHALL include `thinking: {type: "adaptive"}` in requests to Opus 4.6 and Sonnet 4.6 models. The system SHALL parse `thinking` content blocks from responses and emit them as `ThinkingDelta` messages.

#### Scenario: Thinking blocks in response
- **WHEN** the API returns thinking content blocks
- **THEN** the system emits `ThinkingDelta` messages that the consumer can display or discard

### Requirement: Handle API errors with retry
The system SHALL retry requests on 429 (rate limit), 500 (server error), and 529 (overloaded) status codes using exponential backoff with jitter. The system SHALL NOT retry 400, 401, 403, 404, or 413 errors.

#### Scenario: Rate limited
- **WHEN** the API returns 429
- **THEN** the system waits using the `retry-after` header (or exponential backoff) and retries the request

#### Scenario: Bad request
- **WHEN** the API returns 400
- **THEN** the system emits an `Error` message on the output channel without retrying

### Requirement: Support multi-turn conversation
The system SHALL maintain the full message history (alternating user/assistant roles) and send it with each request. The system SHALL append the full `response.content` (not just text) to preserve tool_use blocks and compaction state.

#### Scenario: Multi-turn with tool use
- **WHEN** a previous turn included tool_use and tool_result messages
- **THEN** the next request includes the full conversation history with all content block types preserved


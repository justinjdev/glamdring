package api

import (
	"encoding/json"
	"fmt"
)

// MessageRequest is the request body for POST /v1/messages.
type MessageRequest struct {
	Model     string            `json:"model"`
	MaxTokens int               `json:"max_tokens"`
	Messages  []RequestMessage  `json:"messages"`
	System    any               `json:"system,omitempty"`
	Tools     []json.RawMessage `json:"tools,omitempty"`
	Stream    bool              `json:"stream"`
	Thinking  *ThinkingConfig   `json:"thinking,omitempty"`
}

// ThinkingConfig controls extended thinking behavior.
type ThinkingConfig struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// SystemBlock is a structured system prompt block with optional cache control.
// Use []SystemBlock as the System field in MessageRequest for cache control support.
type SystemBlock struct {
	Type         string        `json:"type"`
	Text         string        `json:"text,omitempty"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// CacheControl specifies caching behavior for a content block.
type CacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

// RequestMessage is a single message in the conversation history.
type RequestMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string for simple text, []ContentBlock for structured content
}

// MessageResponse is the full response from the Messages API (non-streaming).
// In streaming mode, this is delivered via the message_start event.
type MessageResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence *string        `json:"stop_sequence"`
	Usage        Usage          `json:"usage"`
}

// ContentBlock represents a single content block in a message.
// The Type field determines which other fields are populated.
type ContentBlock struct {
	Type string `json:"type"`

	// type: "text"
	Text string `json:"text,omitempty"`

	// type: "thinking"
	Thinking  string `json:"thinking,omitempty"`
	Signature string `json:"signature,omitempty"`

	// type: "tool_use"
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// type: "tool_result"
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`

	// type: "image"
	Source *ImageSource `json:"source,omitempty"`
}

// ImageSource represents the source data for an image content block.
type ImageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // "image/png"
	Data      string `json:"data"`       // base64-encoded image data
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// StreamEvent represents a single server-sent event from the streaming API.
type StreamEvent struct {
	Type string

	// message_start
	Message *MessageResponse

	// content_block_start
	Index        int
	ContentBlock *ContentBlock

	// content_block_delta
	Delta *Delta

	// message_delta
	StopReason string
	Usage      *Usage

	// error
	Err error
}

// Delta is the delta payload inside content_block_delta and message_delta events.
type Delta struct {
	Type string `json:"type"`

	// text_delta
	Text string `json:"text,omitempty"`

	// thinking_delta
	Thinking string `json:"thinking,omitempty"`

	// signature_delta
	Signature string `json:"signature,omitempty"`

	// input_json_delta
	PartialJSON string `json:"partial_json,omitempty"`

	// message_delta fields
	StopReason string `json:"stop_reason,omitempty"`
}

// sseMessageStart is the JSON payload for message_start events.
type sseMessageStart struct {
	Type    string          `json:"type"`
	Message MessageResponse `json:"message"`
}

// sseContentBlockStart is the JSON payload for content_block_start events.
type sseContentBlockStart struct {
	Type         string       `json:"type"`
	Index        int          `json:"index"`
	ContentBlock ContentBlock `json:"content_block"`
}

// sseContentBlockDelta is the JSON payload for content_block_delta events.
type sseContentBlockDelta struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta Delta  `json:"delta"`
}

// sseContentBlockStop is the JSON payload for content_block_stop events.
type sseContentBlockStop struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

// sseMessageDelta is the JSON payload for message_delta events.
type sseMessageDelta struct {
	Type  string `json:"type"`
	Delta Delta  `json:"delta"`
	Usage Usage  `json:"usage"`
}

// sseError is the JSON payload for error events.
type sseError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// APIError represents a structured error response from the Claude API.
type APIError struct {
	StatusCode int
	Type       string
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("api error %d (%s): %s", e.StatusCode, e.Type, e.Message)
}

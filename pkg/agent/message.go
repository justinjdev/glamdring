package agent

// MessageType identifies the kind of message emitted by the agent loop.
type MessageType int

const (
	MessageTextDelta MessageType = iota
	MessageThinkingDelta
	MessageToolCall
	MessageToolResult
	MessageToolOutputDelta
	MessagePermissionRequest
	MessagePermissionResponse
	MessageError
	MessageMaxTurnsReached
	MessageDone
)

// Message is the unit of communication from the agent loop to the consumer.
type Message struct {
	Type MessageType

	// TextDelta / ThinkingDelta
	Text string

	// ToolCall
	ToolName  string
	ToolID    string
	ToolInput map[string]any

	// ToolResult
	ToolOutput  string
	ToolIsError bool

	// PermissionRequest
	PermissionSummary  string
	PermissionResponse chan PermissionAnswer

	// Error
	Err error

	// Usage (updated on MessageDone)
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int

	// LastRequestInputTokens is the input token count from the most recent
	// API call, representing the current context window snapshot.
	LastRequestInputTokens int
}

// PermissionAnswer is the user's response to a permission prompt.
type PermissionAnswer int

const (
	PermissionApprove PermissionAnswer = iota
	PermissionAlwaysApprove
	PermissionDeny
)

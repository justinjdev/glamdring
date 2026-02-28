package teams

// ContextCompactor summarizes conversation history at phase boundaries.
type ContextCompactor interface {
	Compact(conversationHistory string) (string, error)
}

// NoOpCompactor returns the input unchanged. Used when no compaction is configured.
type NoOpCompactor struct{}

// Compact returns the input unchanged.
func (NoOpCompactor) Compact(history string) (string, error) {
	return history, nil
}

// CompactFunc is the signature for a function that summarizes conversation text.
type CompactFunc func(history string) (string, error)

// CallbackCompactor delegates compaction to an injected function. This allows
// the caller (e.g., main.go) to wire in API access without the teams package
// importing the API client directly.
type CallbackCompactor struct {
	Fn CompactFunc
}

// Compact delegates to the injected function.
func (c CallbackCompactor) Compact(history string) (string, error) {
	if c.Fn == nil {
		return history, nil
	}
	return c.Fn(history)
}

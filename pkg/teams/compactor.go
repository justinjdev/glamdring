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

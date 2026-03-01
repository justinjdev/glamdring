package teams

import "fmt"

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

// ArchivingCompactor stores the raw conversation history in a ContextCache
// before delegating to an inner compactor. This provides an escape hatch to
// recover the original context if compaction loses important information.
type ArchivingCompactor struct {
	Inner ContextCompactor
	Cache ContextCache
}

// Compact archives the raw input under a key derived from its length and a
// monotonic counter, then delegates to the inner compactor.
func (a ArchivingCompactor) Compact(history string) (string, error) {
	if a.Cache != nil {
		key := fmt.Sprintf("archive-%d", len(history))
		a.Cache.Store(key, history)
	}
	if a.Inner == nil {
		return history, nil
	}
	return a.Inner.Compact(history)
}

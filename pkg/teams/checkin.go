package teams

import "sync"

// InMemoryCheckinTracker is an in-memory implementation of CheckinTracker.
type InMemoryCheckinTracker struct {
	mu     sync.Mutex
	counts map[string]int
}

// NewInMemoryCheckinTracker creates a new InMemoryCheckinTracker.
func NewInMemoryCheckinTracker() *InMemoryCheckinTracker {
	return &InMemoryCheckinTracker{
		counts: make(map[string]int),
	}
}

// Increment adds 1 to the agent's counter and returns the new count.
func (t *InMemoryCheckinTracker) Increment(agentName string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.counts[agentName]++
	return t.counts[agentName]
}

// Reset sets the agent's counter to 0.
func (t *InMemoryCheckinTracker) Reset(agentName string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.counts[agentName] = 0
}

// Count returns the current count for the agent.
func (t *InMemoryCheckinTracker) Count(agentName string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.counts[agentName]
}

// Remove deletes the counter for the agent.
func (t *InMemoryCheckinTracker) Remove(agentName string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.counts, agentName)
}

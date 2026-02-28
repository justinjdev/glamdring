package teams

import (
	"fmt"
	"sync"
)

type agentPhases struct {
	phases  []Phase
	current int
}

// InMemoryPhaseTracker is an in-memory implementation of PhaseTracker.
type InMemoryPhaseTracker struct {
	mu     sync.RWMutex
	agents map[string]*agentPhases
}

// NewInMemoryPhaseTracker creates a new InMemoryPhaseTracker.
func NewInMemoryPhaseTracker() *InMemoryPhaseTracker {
	return &InMemoryPhaseTracker{
		agents: make(map[string]*agentPhases),
	}
}

// SetPhases stores the phase list for an agent, starting at index 0.
func (t *InMemoryPhaseTracker) SetPhases(agentName string, phases []Phase) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.agents[agentName] = &agentPhases{phases: phases, current: 0}
}

// Current returns the current phase and its index for the named agent.
func (t *InMemoryPhaseTracker) Current(agentName string) (*Phase, int, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	ap, exists := t.agents[agentName]
	if !exists {
		return nil, 0, fmt.Errorf("agent %q not found in phase tracker", agentName)
	}
	if len(ap.phases) == 0 {
		return nil, 0, fmt.Errorf("agent %q has no phases", agentName)
	}
	p := ap.phases[ap.current]
	return &p, ap.current, nil
}

// Advance moves the agent to the next phase. Returns an error if already at
// the last phase or if the agent is not found.
func (t *InMemoryPhaseTracker) Advance(agentName string) (*Phase, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	ap, exists := t.agents[agentName]
	if !exists {
		return nil, fmt.Errorf("agent %q not found in phase tracker", agentName)
	}
	if ap.current >= len(ap.phases)-1 {
		return nil, fmt.Errorf("agent %q is already at the last phase", agentName)
	}
	ap.current++
	p := ap.phases[ap.current]
	return &p, nil
}

// AdvanceTo sets the agent's current phase to the named phase. Returns an
// error if the phase name is not found.
func (t *InMemoryPhaseTracker) AdvanceTo(agentName string, phaseName string) (*Phase, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	ap, exists := t.agents[agentName]
	if !exists {
		return nil, fmt.Errorf("agent %q not found in phase tracker", agentName)
	}
	for i, p := range ap.phases {
		if p.Name == phaseName {
			ap.current = i
			return &ap.phases[i], nil
		}
	}
	return nil, fmt.Errorf("phase %q not found for agent %q", phaseName, agentName)
}

// Remove deletes the phase tracking entry for the named agent.
func (t *InMemoryPhaseTracker) Remove(agentName string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.agents, agentName)
}

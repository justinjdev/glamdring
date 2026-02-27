package agents

import (
	"sort"
)

// Registry provides lookup of custom agent definitions.
type Registry struct {
	agents map[string]AgentDefinition
}

// NewRegistry creates a Registry from a list of discovered agent definitions.
func NewRegistry(agents []AgentDefinition) *Registry {
	m := make(map[string]AgentDefinition, len(agents))
	for _, a := range agents {
		// First entry wins (project-level should come before user-level
		// in the input from Discover).
		if _, exists := m[a.Name]; !exists {
			m[a.Name] = a
		}
	}
	return &Registry{agents: m}
}

// Get returns the agent definition with the given name, if it exists.
func (r *Registry) Get(name string) (AgentDefinition, bool) {
	a, ok := r.agents[name]
	return a, ok
}

// Names returns a sorted list of all agent names.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

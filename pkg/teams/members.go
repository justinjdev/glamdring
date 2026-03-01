package teams

import (
	"fmt"
	"sort"
	"sync"
)

// InMemoryMemberRegistry is an in-memory implementation of MemberRegistry.
type InMemoryMemberRegistry struct {
	mu      sync.RWMutex
	members map[string]*Member
}

// NewInMemoryMemberRegistry creates a new InMemoryMemberRegistry.
func NewInMemoryMemberRegistry() *InMemoryMemberRegistry {
	return &InMemoryMemberRegistry{
		members: make(map[string]*Member),
	}
}

// Add registers a new member. Returns an error if the name is already taken
// or if required fields are missing.
func (r *InMemoryMemberRegistry) Add(member Member) error {
	if member.Name == "" {
		return fmt.Errorf("member name must not be empty")
	}
	if !member.Status.Valid() {
		return fmt.Errorf("invalid member status %q", member.Status)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.members[member.Name]; exists {
		return fmt.Errorf("member %q already exists", member.Name)
	}
	m := member
	r.members[member.Name] = &m
	return nil
}

// Remove removes a member by name. Returns an error if not found.
func (r *InMemoryMemberRegistry) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.members[name]; !exists {
		return fmt.Errorf("member %q not found", name)
	}
	delete(r.members, name)
	return nil
}

// Get returns a copy of the named member. Returns an error if not found.
func (r *InMemoryMemberRegistry) Get(name string) (*Member, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	m, exists := r.members[name]
	if !exists {
		return nil, fmt.Errorf("member %q not found", name)
	}
	copy := *m
	return &copy, nil
}

// SetStatus updates a member's status. Returns an error if not found or if
// the status is invalid.
func (r *InMemoryMemberRegistry) SetStatus(name string, status MemberStatus) error {
	if !status.Valid() {
		return fmt.Errorf("invalid member status %q", status)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	m, exists := r.members[name]
	if !exists {
		return fmt.Errorf("member %q not found", name)
	}
	m.Status = status
	return nil
}

// List returns copies of all members sorted alphabetically by name.
func (r *InMemoryMemberRegistry) List() []Member {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Member, 0, len(r.members))
	for _, m := range r.members {
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

// Count returns the total number of members.
func (r *InMemoryMemberRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.members)
}

// FirstMember returns the name of the alphabetically first member, or false
// if the registry is empty.
func (r *InMemoryMemberRegistry) FirstMember() (string, bool) {
	members := r.List()
	if len(members) == 0 {
		return "", false
	}
	return members[0].Name, true
}

// ActiveCount returns the number of members whose status is not shutdown.
func (r *InMemoryMemberRegistry) ActiveCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, m := range r.members {
		if m.Status != MemberStatusShutdown {
			count++
		}
	}
	return count
}

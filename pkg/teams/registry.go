package teams

import (
	"fmt"
	"sync"
)

// ManagerRegistry is a global map of active TeamManager instances, keyed by team name.
type ManagerRegistry struct {
	mu       sync.RWMutex
	managers map[string]*TeamManager
}

// NewManagerRegistry creates a new ManagerRegistry.
func NewManagerRegistry() *ManagerRegistry {
	return &ManagerRegistry{managers: make(map[string]*TeamManager)}
}

// Create creates a new TeamManager for the given config and registers it.
// taskDir is the directory for file-based task storage.
func (r *ManagerRegistry) Create(cfg TeamConfig, taskDir string) (*TeamManager, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.managers[cfg.Name]; exists {
		return nil, fmt.Errorf("team %q already exists", cfg.Name)
	}
	mgr, err := NewTeamManager(cfg, taskDir)
	if err != nil {
		return nil, err
	}
	r.managers[cfg.Name] = mgr
	return mgr, nil
}

// Get returns the TeamManager for the named team, or nil if not found.
func (r *ManagerRegistry) Get(name string) *TeamManager {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.managers[name]
}

// Delete removes a team from the registry. The team must have no active members.
func (r *ManagerRegistry) Delete(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	mgr, exists := r.managers[name]
	if !exists {
		return fmt.Errorf("team %q not found", name)
	}
	if err := mgr.Delete(); err != nil {
		return err
	}
	delete(r.managers, name)
	return nil
}

// List returns the names of all active teams.
func (r *ManagerRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.managers))
	for name := range r.managers {
		names = append(names, name)
	}
	return names
}

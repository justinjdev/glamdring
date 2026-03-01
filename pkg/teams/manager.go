package teams

import (
	"errors"
	"fmt"
)

// TeamManager composes all team subsystems into a single coordinator.
type TeamManager struct {
	Config   TeamConfig
	Members  MemberRegistry
	Tasks    TaskStorage
	Messages MessageTransport
	Locks    LockManager
	Context  ContextCache
	Phases   PhaseTracker
	Checkins CheckinTracker
}

// NewTeamManager creates a TeamManager with all subsystem instances.
// taskDir is the directory used for file-based task storage.
func NewTeamManager(cfg TeamConfig, taskDir string) (*TeamManager, error) {
	tasks, err := NewFileTaskStorage(taskDir)
	if err != nil {
		return nil, fmt.Errorf("create task storage: %w", err)
	}

	return &TeamManager{
		Config:   cfg,
		Members:  NewInMemoryMemberRegistry(),
		Tasks:    tasks,
		Messages: NewChannelTransport(),
		Locks:    NewInMemoryLockManager(),
		Context:  NewInMemoryContextCache(),
		Phases:   NewInMemoryPhaseTracker(),
		Checkins: NewInMemoryCheckinTracker(),
	}, nil
}

// CleanupAgent performs cross-cutting teardown for the named agent:
// sets status to shutdown, unsubscribes from messages, releases all locks,
// and removes phase and checkin tracking. All cleanup steps are attempted
// even if earlier ones fail, to avoid resource leaks.
func (m *TeamManager) CleanupAgent(name string) error {
	var errs []error
	if err := m.Members.SetStatus(name, MemberStatusShutdown); err != nil {
		errs = append(errs, fmt.Errorf("set member status: %w", err))
	}
	m.Messages.Unsubscribe(name)
	m.Locks.ReleaseAll(name)
	m.Phases.Remove(name)
	m.Checkins.Remove(name)
	return errors.Join(errs...)
}

// Delete verifies no active members remain and tears down the team.
// Returns an error if any members are still active.
func (m *TeamManager) Delete() error {
	if count := m.Members.ActiveCount(); count > 0 {
		return fmt.Errorf("cannot delete team: %d active member(s) remaining", count)
	}
	return nil
}

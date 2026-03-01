package teams

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FileTaskStorage is a JSON file-based implementation of TaskStorage.
// Each task is stored as a separate JSON file in a directory.
type FileTaskStorage struct {
	dir   string
	mu    sync.Mutex
	nextN int
}

// NewFileTaskStorage creates a new FileTaskStorage backed by the given directory.
// The directory is created if it does not exist. Existing files are scanned to
// determine the next available task ID.
func NewFileTaskStorage(dir string) (*FileTaskStorage, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create task dir: %w", err)
	}

	s := &FileTaskStorage{dir: dir, nextN: 1}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read task dir: %w", err)
	}

	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		idStr := strings.TrimSuffix(name, ".json")
		n, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		if n >= s.nextN {
			s.nextN = n + 1
		}
	}

	return s, nil
}

// nextID returns the next available task ID as a string.
func (s *FileTaskStorage) nextID() string {
	id := strconv.Itoa(s.nextN)
	s.nextN++
	return id
}

// Create persists a new task. It assigns an ID and sets timestamps.
func (s *FileTaskStorage) Create(task Task) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task.ID = s.nextID()
	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now

	if err := s.writeTaskLocked(&task); err != nil {
		return nil, err
	}
	return &task, nil
}

// Get reads a task by ID.
func (s *FileTaskStorage) Get(id string) (*Task, error) {
	return s.readTask(id)
}

// Update applies changes to an existing task. If ExpectedOwner is set, a
// compare-and-swap check is performed against the current owner.
func (s *FileTaskStorage) Update(id string, update TaskUpdate) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, err := s.readTaskLocked(id)
	if err != nil {
		return nil, err
	}

	// CAS ownership check.
	if update.Owner != nil && update.ExpectedOwner != nil {
		if task.Owner != *update.ExpectedOwner {
			return nil, fmt.Errorf("task ownership conflict: expected owner %q but current owner is %q", *update.ExpectedOwner, task.Owner)
		}
	}

	// Reject claiming a blocked task.
	if update.Owner != nil && *update.Owner != "" && len(task.BlockedBy) > 0 {
		return nil, fmt.Errorf("cannot claim task %s: blocked by %v", id, task.BlockedBy)
	}

	if update.Status != nil {
		task.Status = *update.Status
	}
	if update.Subject != nil {
		task.Subject = *update.Subject
	}
	if update.Description != nil {
		task.Description = *update.Description
	}
	if update.Owner != nil {
		task.Owner = *update.Owner
	}
	if len(update.AddBlocks) > 0 {
		task.Blocks = appendUnique(task.Blocks, update.AddBlocks)
	}
	if len(update.AddBlockedBy) > 0 {
		task.BlockedBy = appendUnique(task.BlockedBy, update.AddBlockedBy)
	}
	if update.Scope != nil {
		task.Scope = update.Scope
	}

	task.UpdatedAt = time.Now()

	if err := s.writeTaskLocked(task); err != nil {
		return nil, err
	}

	// When a task is completed, remove it from other tasks' BlockedBy lists.
	if update.Status != nil && *update.Status == TaskStatusCompleted {
		if err := s.clearBlockedByLocked(id); err != nil {
			return task, fmt.Errorf("task %s completed but failed to unblock dependents: %w", id, err)
		}
	}

	return task, nil
}

// List returns summaries of all non-deleted tasks.
func (s *FileTaskStorage) List() []TaskSummary {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		log.Printf("warning: failed to read task directory %s: %v", s.dir, err)
		return nil
	}

	var out []TaskSummary
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		idStr := strings.TrimSuffix(e.Name(), ".json")
		task, err := s.readTaskLocked(idStr)
		if err != nil {
			log.Printf("warning: skipping unreadable task file %s: %v", e.Name(), err)
			continue
		}
		if task.Status == TaskStatusDeleted {
			continue
		}
		out = append(out, TaskSummary{
			ID:        task.ID,
			Subject:   task.Subject,
			Status:    task.Status,
			Owner:     task.Owner,
			BlockedBy: task.BlockedBy,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		ni, _ := strconv.Atoi(out[i].ID)
		nj, _ := strconv.Atoi(out[j].ID)
		return ni < nj
	})

	return out
}

// Delete removes a task's file from disk.
func (s *FileTaskStorage) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.taskPath(id)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("task %q not found", id)
		}
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
}

func (s *FileTaskStorage) taskPath(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func (s *FileTaskStorage) readTask(id string) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readTaskLocked(id)
}

func (s *FileTaskStorage) readTaskLocked(id string) (*Task, error) {
	data, err := os.ReadFile(s.taskPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("task %q not found", id)
		}
		return nil, fmt.Errorf("read task: %w", err)
	}
	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("unmarshal task: %w", err)
	}
	return &task, nil
}

func (s *FileTaskStorage) writeTaskLocked(task *Task) error {
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal task: %w", err)
	}
	// Write to temp file then rename for atomic updates. This prevents
	// partial writes from corrupting task files on crash.
	tmpPath := s.taskPath(task.ID) + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write task: %w", err)
	}
	if err := os.Rename(tmpPath, s.taskPath(task.ID)); err != nil {
		return fmt.Errorf("rename task: %w", err)
	}
	return nil
}

// clearBlockedByLocked removes the given task ID from all other tasks' BlockedBy slices.
func (s *FileTaskStorage) clearBlockedByLocked(completedID string) error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("read task dir: %w", err)
	}
	var errs []error
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		idStr := strings.TrimSuffix(e.Name(), ".json")
		if idStr == completedID {
			continue
		}
		task, err := s.readTaskLocked(idStr)
		if err != nil {
			errs = append(errs, fmt.Errorf("read task %s: %w", idStr, err))
			continue
		}
		filtered := removeFromSlice(task.BlockedBy, completedID)
		if len(filtered) != len(task.BlockedBy) {
			task.BlockedBy = filtered
			task.UpdatedAt = time.Now()
			if err := s.writeTaskLocked(task); err != nil {
				errs = append(errs, fmt.Errorf("write task %s: %w", idStr, err))
			}
		}
	}
	return errors.Join(errs...)
}

func appendUnique(existing, additions []string) []string {
	set := make(map[string]bool, len(existing))
	for _, v := range existing {
		set[v] = true
	}
	for _, v := range additions {
		if !set[v] {
			existing = append(existing, v)
			set[v] = true
		}
	}
	return existing
}

func removeFromSlice(s []string, val string) []string {
	out := make([]string, 0, len(s))
	for _, v := range s {
		if v != val {
			out = append(out, v)
		}
	}
	return out
}
